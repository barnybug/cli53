#!/usr/bin/env python

# cli53
# Command line script to administer the Amazon Route 53 dns service

import os, sys
import re
import itertools
from cStringIO import StringIO
from time import sleep

# needs latest boto from github: http://github.com/boto/boto
# git clone git://github.com/boto/boto
try:
    import boto.route53, boto.jsonresponse, boto.exception
except ImportError:
    print "Please install latest boto:"
    print "git clone boto && cd boto && python setup.py install"
    sys.exit(-1)

import argparse
from argparse import ArgumentTypeError
from types import StringTypes
try:
    import xml.etree.ElementTree as et
except ImportError:
    import elementtree.ElementTree as et

try:
    import dns.zone, dns.rdataset, dns.node, dns.rdtypes, dns.rdataclass
    import dns.rdtypes.ANY.SOA, dns.rdtypes.ANY.SPF
    import dns.rdtypes.ANY.TXT, dns.rdtypes.IN.A, dns.rdtypes.IN.AAAA
    import dns.rdtypes.mxbase, dns.rdtypes.nsbase
except ImportError:
    print "Please install dnspython:"
    print "easy_install dnspython"
    sys.exit(-1)

# Custom MX class to prevent changing values
class MX(dns.rdtypes.mxbase.MXBase):
    def to_text(self, **kw):
        return '%d %s' % (self.preference, self.exchange)

# Custom base class to prevent changing values
class CustomBase(dns.rdtypes.nsbase.NSBase):
    def to_text(self, **kw):
        return self.target

class CNAME(CustomBase):
    pass
class NS(CustomBase):
    pass
class SRV(CustomBase):
    pass
class PTR(CustomBase):
    pass
class SPF(CustomBase):
    pass

try:
    r53 = boto.route53.Route53Connection()
except boto.exception.NoAuthHandlerFound:
    print 'Please configure your AWS credentials, either through environment variables or'
    print '~/.boto config file.'
    print 'e.g.'
    print 'export AWS_ACCESS_KEY_ID=XXXXXXXXXXXXXX'
    print 'export AWS_SECRET_ACCESS_KEY=XXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
    print 'or in ~/.boto:'
    print '[Credentials]'
    print 'aws_access_key_id = XXXXXXXXXXXXXX'
    print 'aws_secret_access_key = XXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
    print
    print  'See: http://code.google.com/p/boto/wiki/BotoConfig'
    sys.exit(-1)

def pprint(obj, findent='', indent=''):
    if isinstance(obj, StringTypes):
        print '%s%s' % (findent, obj)
    elif isinstance(obj, boto.jsonresponse.Element):
        i = findent
        for k, v in obj.iteritems():
            if k in ('IsTruncated', 'MaxItems'): continue
            if isinstance(v, StringTypes):
                print '%s%s: %s' % (i, k, v)
            else:
                print '%s%s:' % (i, k)
                pprint(v, indent+'  ', indent+'  ')
            i = indent
    elif isinstance(obj, (boto.jsonresponse.ListElement, list)):
        i = findent
        for v in obj:
            pprint(v, i+'- ', i+'  ')
            i = indent
    else:
        raise ValueError, 'Cannot pprint type %s' % type(obj)

def cmd_list(args):
    ret = r53.get_all_hosted_zones()
    pprint(ret.ListHostedZonesResponse)

def cmd_info(args):
    ret = r53.get_hosted_zone(args.zone)
    pprint(ret.GetHostedZoneResponse)

def text_element(parent, name, text):
    el = et.SubElement(parent, name)
    el.text = text

def is_root_soa_or_ns(name, rdataset):
    rt = dns.rdatatype.to_text(rdataset.rdtype)
    return (rt in ('SOA', 'NS') and name.to_text() == '@')

def paginate(iterable, size):
  it = iter(iterable)
  item = list(itertools.islice(it, size))
  while item:
    yield item
    item = list(itertools.islice(it, size))

class BindToR53Formatter(object):
    def _build_list(self, zone, exclude=None):
        li = []
        for name, node in zone.items():
            for rdataset in node.rdatasets:
                if not exclude or not exclude(name, rdataset):
                    li.append((name, rdataset))
        return li

    def create_all(self, zone, old_zone=None, exclude=None):
        creates = self._build_list(zone, exclude)
        deletes = []
        if old_zone:
            deletes = self._build_list(old_zone, exclude)
        return self._xml_changes(zone, creates=creates, deletes=deletes)

    def dump_xml(self, zone, exclude=None):

        re_awsalias = re.compile(r'^AWSALIAS')
        # preprocess; this is annoying but necessary to support our little
        # TXT record shim: doing it inside dnspython is just painful
        rr_data = {}
        for rrname in zone.keys():
            rr_name = rrname.derelativize(zone.origin).to_text()
            rr_data[rr_name] = {}
            for rdataset in zone[rrname].rdatasets:
                rr_type = dns.rdatatype.to_text(rdataset.rdtype)
                rr_data[rr_name][rr_type] = {}
                rr_data[rr_name][rr_type]['TTL'] = str(rdataset.ttl)
                rr_data[rr_name][rr_type]['RRS'] = []
                for rdtype in rdataset.items:
                    rr_data[rr_name][rr_type]['RRS'].append(rdtype.to_text(origin=zone.origin,
                        relativize=False))

        # now deal with the ugliness of aws alias records
        for rr_name in rr_data:
            # first, convert any AWSALIAS txt records into A records
            if 'TXT' in rr_data[rr_name]:
                rr_vals_to_delete = []
                for rr_value in rr_data[rr_name]['TXT']['RRS']:
                    if re_awsalias.search(unquote(rr_value)):
                        (_, hosted_zone_id, dns_name) = unquote(rr_value).split(':')
                        # remove the awsalias from the TXT record set
                        rr_vals_to_delete.append(rr_value)
                        # add as an A record with an alias target
                        if 'A' not in rr_data[rr_name]:
                            rr_data[rr_name]['A'] = {}
                        rr_data[rr_name]['A']['AliasTarget'] = {}
                        rr_data[rr_name]['A']['AliasTarget']['HostedZoneId'] = hosted_zone_id
                        rr_data[rr_name]['A']['AliasTarget']['DNSName'] = dns_name
                for rr_value in rr_vals_to_delete:
                    del(rr_data[rr_name]['TXT']['RRS'][
                        rr_data[rr_name]['TXT']['RRS'].index(rr_value)])
                # if we've emptied the TXT set, delete it
                if not rr_data[rr_name]['TXT']['RRS']:
                    del rr_data[rr_name]['TXT']
            # now make sure there's no existing A record for that RR
            if 'A' in rr_data[rr_name]:
                if 'RRS' in rr_data[rr_name]['A'] and 'AliasTarget' in rr_data[rr_name]['A']:
                    raise ValueError(
                        'You cannot have both a static A record and an AWSALIAS'
                        ' at the same RR node: %s' % rr_name)

        # now spit it all back out as XML
        resource_record_sets = et.Element('ResourceRecordSets',
                xmlns=boto.route53.Route53Connection.XMLNameSpace)

        for rr_name in rr_data:
            for rr_type in rr_data[rr_name]:
                resource_record_set = et.SubElement(resource_record_sets, 'ResourceRecordSet')
                text_element(resource_record_set, 'Name', rr_name)
                text_element(resource_record_set, 'Type', rr_type)
                if 'AliasTarget' in rr_data[rr_name][rr_type]:
                    alias_target = et.SubElement(resource_record_set, 'AliasTarget')
                    text_element(alias_target, 'HostedZoneId',
                            rr_data[rr_name][rr_type]['AliasTarget']['HostedZoneId'])
                    text_element(alias_target, 'DNSName',
                            rr_data[rr_name][rr_type]['AliasTarget']['DNSName'])
                else:
                    text_element(resource_record_set, 'TTL', rr_data[rr_name][rr_type]['TTL'])
                    resource_records = et.SubElement(resource_record_set, 'ResourceRecords')
                    for rr_value in rr_data[rr_name][rr_type]['RRS']:
                        resource_record = et.SubElement(resource_records, 'ResourceRecord')
                        text_element(resource_record, 'Value', rr_value)

        out = StringIO()
        et.ElementTree(resource_record_sets).write(out)
        return out.getvalue()


    def delete_all(self, zone, exclude=None):
        return self._xml_changes(zone, deletes=self._build_list(zone, exclude))

    def create_record(self, zone, name, rdataset):
        return self._xml_changes(zone, creates=[(name,rdataset)])

    def delete_record(self, zone, name, rdataset):
        return self._xml_changes(zone, deletes=[(name,rdataset)])

    def replace_record(self, zone, name, rdataset_old, rdataset_new):
        return self._xml_changes(zone, creates=[(name,rdataset_new)], deletes=[(name,rdataset_old)])

    def _xml_changes(self, zone, creates=[], deletes=[]):
        for page in paginate(self._iter_changes(creates, deletes), 100):
            yield self._batch_change(zone, page)

    def _iter_changes(self, creates, deletes):
        for chg, rdatasets in (('DELETE', deletes), ('CREATE', creates)):
            for name, rdataset in rdatasets:
                yield chg, name, rdataset

    def _batch_change(self, zone, chgs):
        root = et.Element('ChangeResourceRecordSetsRequest', xmlns=boto.route53.Route53Connection.XMLNameSpace)
        change_batch = et.SubElement(root, 'ChangeBatch')
        changes = et.SubElement(change_batch, 'Changes')

        for chg, name, rdataset in chgs:
            change = et.SubElement(changes, 'Change')
            text_element(change, 'Action', chg)
            rrset = et.SubElement(change, 'ResourceRecordSet')
            text_element(rrset, 'Name', name.derelativize(zone.origin).to_text())
            text_element(rrset, 'Type', dns.rdatatype.to_text(rdataset.rdtype))
            text_element(rrset, 'TTL', str(rdataset.ttl))
            rrs = et.SubElement(rrset, 'ResourceRecords')
            for rdtype in rdataset.items:
                rr = et.SubElement(rrs, 'ResourceRecord')
                text_element(rr, 'Value', rdtype.to_text(origin=zone.origin, relativize=False))

        out = StringIO()
        et.ElementTree(root).write(out)
        return out.getvalue()

class R53ToBindFormatter(object):
    def get_all_rrsets(self, r53, ghz, zone):
        rrsets = r53.get_all_rrsets(zone, maxitems=10)
        return self.convert(ghz, rrsets)

    def convert(self, info, rrsets, z=None):
        if not z:
            origin = info.HostedZone.Name
            z = dns.zone.Zone(dns.name.from_text(origin))

        for rrset in rrsets:
            name = rrset.name
            if '\\052' in name:
                # * char seems to confuse Amazon and is returned as \\052
                name = name.replace('\\052', '*')
            rtype = rrset.type
            ttl = int(rrset.ttl)

            rdataset = _create_rdataset(rtype, ttl, rrset.resource_records)
            node = z.get_node(name, create=True)
            node.replace_rdataset(rdataset)

        return z

re_quoted = re.compile(r'^".*"$')
re_backslash = re.compile(r'\\(.)')
def unquote(v):
    v = v[1:-1]
    v = re_backslash.sub('\\1', v)
    return v

def _create_rdataset(rtype, ttl, values):
    rdataset = dns.rdataset.Rdataset(dns.rdataclass.IN, dns.rdatatype.from_text(rtype))
    rdataset.ttl = ttl
    for value in values:
        rdtype = None
        if rtype == 'A':
            rdtype = dns.rdtypes.IN.A.A(dns.rdataclass.IN, dns.rdatatype.A, value)
        elif rtype == 'AAAA':
            rdtype = dns.rdtypes.IN.AAAA.AAAA(dns.rdataclass.IN, dns.rdatatype.AAAA, value)
        elif rtype == 'CNAME':
            rdtype = CNAME(dns.rdataclass.ANY, dns.rdatatype.CNAME, value)
        elif rtype == 'SOA':
            mname, rname, serial, refresh, retry, expire, minimum = value.split()
            mname = dns.name.from_text(mname)
            rname = dns.name.from_text(rname)
            serial = int(serial)
            refresh = int(refresh)
            retry = int(retry)
            expire = int(expire)
            minimum = int(minimum)
            rdtype = dns.rdtypes.ANY.SOA.SOA(dns.rdataclass.ANY, dns.rdatatype.SOA, mname, rname, serial, refresh, retry, expire, minimum)
        elif rtype == 'NS':
            rdtype = NS(dns.rdataclass.ANY, dns.rdatatype.NS, value)
        elif rtype == 'MX':
            try:
                pref, ex = value.split()
                pref = int(pref)
            except ValueError:
                raise ValueError, 'mx records required two parts: priority name'
            rdtype = MX(dns.rdataclass.ANY, dns.rdatatype.MX, pref, ex)
        elif rtype == 'PTR':
            rdtype = PTR(dns.rdataclass.ANY, dns.rdatatype.PTR, value)
        elif rtype == 'SPF':
            rdtype = SPF(dns.rdataclass.ANY, dns.rdatatype.SPF, value)
        elif rtype == 'SRV':
            rdtype = SRV(dns.rdataclass.IN, dns.rdatatype.SRV, value)
        elif rtype == 'TXT':
            if re_quoted.match(value):
                value = unquote(value)
            rdtype = dns.rdtypes.ANY.TXT.TXT(dns.rdataclass.ANY, dns.rdatatype.TXT, [value])
        else:
            raise ValueError, 'record type %s not handled' % rtype
        rdataset.items.append(rdtype)
    return rdataset

def cmd_xml(args):
    print 'This functionality is no longer available due to changes in the boto library.'

re_dos = re.compile('\r\n$')
re_origin = re.compile(r'\$ORIGIN (\S+)')
re_include = re.compile(r'\$INCLUDE (\S+)')
def cmd_import(args):
    text = []

    def file_parse(zonefile):
        for line in zonefile:
            line = re_dos.sub('\n', line)
            inc = re_include.search(line)
            if inc:
                incfile = open(inc.group(1))
                file_parse(incfile)
                incfile.close()
            else:
                text.append(line)

    file_parse(args.file)

    text = ''.join(text)

    m = re_origin.search(text)
    if not m:
        raise Exception, 'Could not find origin'
    origin = m.group(1)

    zone = dns.zone.from_text(text, origin=origin, check_origin=True)

    if args.dump:
        f = BindToR53Formatter()
        xml = f.dump_xml(zone)
        print xml
        return

    old_zone = None
    if args.replace:
        old_zone = _get_records(args)

    f = BindToR53Formatter()

    if args.editauth:
        exclude_rr = None
    else:
        exclude_rr = is_root_soa_or_ns

    for xml in f.create_all(zone, old_zone=old_zone, exclude=exclude_rr):
        if args.dumpchg:
            print xml
        ret = r53.change_rrsets(args.zone, xml)
        if args.wait:
            wait_for_sync(ret)
        else:
            pprint(ret.ChangeResourceRecordSetsResponse)

re_zone_id = re.compile('^[A-Z0-9]+$')
def Zone(zone):
    if re_zone_id.match(zone):
        return zone
    ret = r53.get_all_hosted_zones()

    hzs = [ hz.Id.replace('/hostedzone/', '') for hz in ret.ListHostedZonesResponse.HostedZones if hz.Name == zone or hz.Name == zone+'.' ]
    if len(hzs) == 1:
        return hzs[0]
    elif len(hzs) > 1:
        raise ArgumentTypeError, 'Zone %r is ambiguous (matches: %s), please specify ID' % (zone, ', '.join(hzs))
    else:
        raise ArgumentTypeError, 'Zone %r not found' % zone

def _get_records(args):
    info = r53.get_hosted_zone(args.zone)
    f = R53ToBindFormatter()
    return f.get_all_rrsets(r53, info.GetHostedZoneResponse, args.zone)

def cmd_export(args):
    zone = _get_records(args)
    print '$ORIGIN %s' % zone.origin.to_text()
    zone.to_file(sys.stdout, relativize=not args.full)

def cmd_create(args):
    ret = r53.create_hosted_zone(args.zone)
    if args.wait:
        wait_for_sync(ret)
    else:
        pprint(ret.CreateHostedZoneResponse)

def cmd_delete(args):
    ret = r53.delete_hosted_zone(args.zone)
    if hasattr(ret, 'ErrorResponse'):
        pprint(ret.ErrorResponse)
    elif args.wait:
        wait_for_sync(ret)
    else:
        pprint(ret.DeleteHostedZoneResponse)

def find_key_nonrecursive(adict, key):
    stack = [adict]
    while stack:
        d = stack.pop()
        if key in d:
            return d[key]
        for k, v in d.iteritems():
            if isinstance(v, dict):
                stack.append(v)

def wait_for_sync(obj):
    waiting = 1
    id = find_key_nonrecursive(obj, 'Id')
    id = id.replace('/change/', '')
    sys.stdout.write("Waiting for change to sync")
    ret = ''
    while waiting:
        ret = r53.get_change(id)
        status = find_key_nonrecursive(ret, 'Status')
        if status == 'INSYNC':
            waiting = 0
        else:
            sys.stdout.write('.')
            sys.stdout.flush()
            sleep(1)
    print "completed"
    pprint(ret.GetChangeResponse)

def cmd_rrcreate(args):
    zone = _get_records(args)
    name = dns.name.from_text(args.rr, zone.origin)
    rdataset = _create_rdataset(args.type, args.ttl, args.values)

    rdataset_old = None
    node = zone.get_node(args.rr)
    if node:
        for rds in node.rdatasets:
            if args.type == dns.rdatatype.to_text(rds.rdtype):
                rdataset_old = rds
                break

    f = BindToR53Formatter()
    if args.replace and rdataset_old:
        parts = f.replace_record(zone, name, rdataset_old, rdataset)
    else:
        parts = f.create_record(zone, name, rdataset)
    for xml in parts:
        ret = r53.change_rrsets(args.zone, xml)
        if args.wait:
            wait_for_sync(ret)
        else:
            print 'Success'
            pprint(ret.ChangeResourceRecordSetsResponse)

def cmd_rrdelete(args):
    zone = _get_records(args)
    name = dns.name.from_text(args.rr, zone.origin)

    node = zone.get_node(args.rr)
    if node:
        if len(node.rdatasets) > 1 and not args.type:
            rtypes = [ dns.rdatatype.to_text(rds.rdtype) for rds in node.rdatasets ]
            print 'Ambigious record - several resource types for record %r found: %s' % (args.rr, ', '.join(rtypes))
        else:
            rdataset = None
            for rds in node.rdatasets:
                if args.type == dns.rdatatype.to_text(rds.rdtype) or not args.type:
                    rdataset = rds
                    break

            if not rdataset:
                print 'Record not found: %s, type: %s' % (args.rr, args.type)
                return

            print 'Deleting %s %s...' % (args.rr, dns.rdatatype.to_text(rds.rdtype))

            f = BindToR53Formatter()
            for xml in f.delete_record(zone, name, rdataset):
                ret = r53.change_rrsets(args.zone, xml)
                if args.wait:
                    wait_for_sync(ret)
                else:
                    print 'Success'
                    pprint(ret.ChangeResourceRecordSetsResponse)
    else:
        print 'Record not found: %s' % args.rr

def cmd_rrpurge(args):
    zone = _get_records(args)
    f = BindToR53Formatter()
    for xml in f.delete_all(zone, exclude=is_root_soa_or_ns):
        ret = r53.change_rrsets(args.zone, xml)
        if args.wait:
            wait_for_sync(ret)
        else:
            pprint(ret.ChangeResourceRecordSetsResponse)

def cmd_rrlist(args):
    zone = _get_records(args)
    print '\t'.join(["host","ttl","cls","type","data"])
    for record_name, record_value in zone.iteritems():
        print '\t'.join(record_value.to_text(record_name).split(' '))

def main():
    connection = boto.route53.Route53Connection()
    parser = argparse.ArgumentParser(description='route53 command line tool')
    subparsers = parser.add_subparsers(help='sub-command help')

    supported_rtypes = ('A', 'AAAA', 'CNAME', 'SOA', 'NS', 'MX', 'PTR', 'SPF', 'SRV', 'TXT')

    parser_list = subparsers.add_parser('list', help='list hosted zones')
    parser_list.set_defaults(func=cmd_list)

    parser_info = subparsers.add_parser('info', help='get details of a hosted zone')
    parser_info.add_argument('zone', type=Zone, help='zone name')
    parser_info.set_defaults(func=cmd_info)

    parser_describe = subparsers.add_parser('xml')
    parser_describe.add_argument('zone', type=Zone, help='zone name')
    parser_describe.set_defaults(func=cmd_xml)

    parser_describe = subparsers.add_parser('export', help='export dns in bind format')
    parser_describe.add_argument('zone', type=Zone, help='zone name')
    parser_describe.add_argument('--full', action='store_true', help='export prefixes as full names')
    parser_describe.set_defaults(func=cmd_export)

    parser_import = subparsers.add_parser('import', help='import dns in bind format')
    parser_import.add_argument('zone', type=Zone, help='zone name')
    parser_import.add_argument('-r', '--replace', action='store_true', help='replace all existing records (use with care!)')
    parser_import.add_argument('-f', '--file', type=argparse.FileType('r'), help='bind file')
    parser_import.add_argument('--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: false)')
    parser_import.add_argument('--editauth', action='store_true', default=False, help='include SOA and NS records from zone file')
    parser_import.add_argument('--dump', action='store_true', help='dump zone file in xml format to stdout')
    parser_import.add_argument('--dumpchg', action='store_true', help='dump xml change request to stdout')
    parser_import.set_defaults(func=cmd_import)

    parser_create = subparsers.add_parser('create', help='create a hosted zone')
    parser_create.add_argument('zone', help='zone name')
    parser_create.add_argument('--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: false)')
    parser_create.set_defaults(func=cmd_create)

    parser_delete = subparsers.add_parser('delete', help='delete a hosted zone')
    parser_delete.add_argument('zone', type=Zone, help='zone name')
    parser_delete.add_argument('--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: false)')
    parser_delete.set_defaults(func=cmd_delete)

    parser_rrcreate = subparsers.add_parser('rrcreate', help='create a resource record')
    parser_rrcreate.add_argument('zone', type=Zone, help='zone name')
    parser_rrcreate.add_argument('rr', help='resource record')
    parser_rrcreate.add_argument('type', choices=supported_rtypes, help='resource record type')
    parser_rrcreate.add_argument('values', nargs='+', help='resource record values')
    parser_rrcreate.add_argument('-x', '--ttl', type=int, default=86400, help='resource record ttl')
    parser_rrcreate.add_argument('-r', '--replace', action='store_true', help='replace any existing record')
    parser_rrcreate.add_argument('--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: false)')
    parser_rrcreate.set_defaults(func=cmd_rrcreate)

    parser_rrdelete = subparsers.add_parser('rrdelete', help='delete a resource record')
    parser_rrdelete.add_argument('zone', type=Zone, help='zone name')
    parser_rrdelete.add_argument('rr', help='resource record')
    parser_rrdelete.add_argument('type', nargs='?', choices=supported_rtypes, help='resource record type')
    parser_rrdelete.add_argument('--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: false)')
    parser_rrdelete.set_defaults(func=cmd_rrdelete)

    parser_rrpurge = subparsers.add_parser('rrpurge', help='purge all resource records')
    parser_rrpurge.add_argument('zone', type=Zone, help='zone name')
    parser_rrpurge.add_argument('--confirm', required=True, action='store_true', help='confirm you definitely want to do this!')
    parser_rrpurge.add_argument('--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: false)')
    parser_rrpurge.set_defaults(func=cmd_rrpurge)

    parser_rrlist = subparsers.add_parser('rrlist', help='list all resource records')
    parser_rrlist.add_argument('zone', type=Zone, help='zone name')
    parser_rrlist.set_defaults(func=cmd_rrlist)

    args = parser.parse_args()
    args.func(args)

if __name__=='__main__':
    main()
