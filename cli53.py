#!/usr/bin/python

# cli53
# Command line script to administer the Amazon Route 53 dns service

import os, sys
import re
from cStringIO import StringIO
from time import sleep

# needs latest boto from github: http://github.com/boto/boto
# git clone git://github.com/boto/boto 
try:
    import boto.route53, boto.jsonresponse
except ImportError:
    print "Please install latest boto:"
    print "git clone boto && cd boto && python setup.py install"
    sys.exit(-1)

import argparse
from argparse import ArgumentError
from types import StringTypes
import xml.etree.ElementTree as et

try:
    import dns.zone, dns.rdataset, dns.node, dns.rdtypes, dns.rdataclass
    import dns.rdtypes.ANY.CNAME, dns.rdtypes.ANY.SOA, dns.rdtypes.ANY.MX, dns.rdtypes.ANY.SPF
    import dns.rdtypes.ANY.TXT, dns.rdtypes.ANY.NS, dns.rdtypes.ANY.PTR, dns.rdtypes.IN.A, dns.rdtypes.IN.AAAA, dns.rdtypes.IN.SRV
except ImportError:
    print "Please install dnspython:"
    print "easy_install dnspython"
    sys.exit(-1)

if not (os.getenv('AWS_ACCESS_KEY_ID') and os.getenv('AWS_SECRET_ACCESS_KEY')):
    print 'Please set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, e.g.:'
    print 'export AWS_ACCESS_KEY_ID=XXXXXXXXXXXXXX'
    print 'export AWS_SECRET_ACCESS_KEY=XXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
    sys.exit(-1)

# Custom MX class to prevent changing values
class MX(dns.rdtypes.mxbase.MXBase):
    def to_text(self, **kw):
        return '%d %s' % (self.preference, self.exchange)

# Custom CNAME class to prevent changing values
class CNAME(dns.rdtypes.nsbase.NSBase):
    def to_text(self, **kw):
        return self.target

r53 = boto.route53.Route53Connection()

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
    
class BindToR53Formatter(object):
    def create_all(self, zone, exclude=None):
        creates = []
        for name, node in zone.items():
            for rdataset in node.rdatasets:
                if not exclude or not exclude(name, rdataset):
                    creates.append((name, rdataset))
        return self._xml_changes(zone, creates=creates)

    def delete_all(self, zone, exclude=None):
        deletes = []
        for name, node in zone.items():
            for rdataset in node.rdatasets:
                if not exclude or not exclude(name, rdataset):
                    deletes.append((name, rdataset))
        return self._xml_changes(zone, deletes=deletes)
        
    def create_record(self, zone, name, rdataset):
        return self._xml_changes(zone, creates=[(name,rdataset)])
        
    def delete_record(self, zone, name, rdataset):
        return self._xml_changes(zone, deletes=[(name,rdataset)])
        
    def replace_record(self, zone, name, rdataset_old, rdataset_new):
        return self._xml_changes(zone, creates=[(name,rdataset_new)], deletes=[(name,rdataset_old)])
        
    def _xml_changes(self, zone, creates=[], deletes=[]):
        root = et.Element('ChangeResourceRecordSetsRequest', xmlns=boto.route53.Route53Connection.XMLNameSpace)
        change_batch = et.SubElement(root, 'ChangeBatch')
        changes = et.SubElement(change_batch, 'Changes')
        
        for chg, rdatasets in (('DELETE', deletes), ('CREATE', creates)):
            for name, rdataset in rdatasets:
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
    def convert(self, info, xml):
        origin = info.HostedZone.Name
        z = dns.zone.Zone(dns.name.from_text(origin))
        
        ns = boto.route53.Route53Connection.XMLNameSpace
        tree = et.fromstring(xml)
        
        for rrsets in tree.findall("{%s}ResourceRecordSets" % ns):
            for rrset in rrsets.findall("{%s}ResourceRecordSet" % ns):
                name = rrset.find('{%s}Name' % ns).text
                if '\\052' in name:
                    # * char seems to confuse Amazon and is returned as \\052
                    name = name.replace('\\052', '*')
                rtype = rrset.find('{%s}Type' % ns).text
                ttl = int(rrset.find('{%s}TTL' % ns).text)
                
                values = [ rr.text for rr in rrset.findall('{%(ns)s}ResourceRecords/{%(ns)s}ResourceRecord/{%(ns)s}Value' % {'ns':ns}) ]
                rdataset = _create_rdataset(rtype, ttl, values)
                node = z.get_node(name, create=True)
                node.replace_rdataset(rdataset)
        
        return z
    
re_quoted = re.compile(r'^".*"$')
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
            rdtype = dns.rdtypes.ANY.NS.NS(dns.rdataclass.ANY, dns.rdatatype.SOA, dns.name.from_text(value))
        elif rtype == 'MX':
            pref, ex = value.split()
            pref = int(pref)
            rdtype = MX(dns.rdataclass.ANY, dns.rdatatype.MX, pref, ex)
        elif rtype == 'PTR':
            rdtype = dns.rdtypes.ANY.PTR.PTR(dns.rdataclass.ANY, dns.rdatatype.PTR, value)
        elif rtype == 'SPF':
            rdtype = dns.rdtypes.ANY.SPF.SPF(dns.rdataclass.ANY, dns.rdatatype.SPF, value)
        elif rtype == 'SRV':
            priority, weight, port, target = value.split()
            priority = int(priority)
            weight = int(weight)
            port = int(port)
            target = dns.name.from_text(target)
            rdtype = dns.rdtypes.IN.SRV.SRV(dns.rdataclass.IN, dns.rdatatype.SRV, priority, weight, port, target)
        elif rtype == 'TXT':
            if re_quoted.match(value):
                value = value[1:-1]
            rdtype = dns.rdtypes.ANY.TXT.TXT(dns.rdataclass.ANY, dns.rdatatype.TXT, value)
        else:
            raise ValueError, 'record type %s not handled' % rtype
        rdataset.items.append(rdtype)
    return rdataset
    
def cmd_xml(args):
    xml = r53.get_all_rrsets(args.zone)
    print xml
    
re_comment = re.compile('\S*;.*$')
re_dos = re.compile('\r\n$')
re_origin = re.compile(r'\$ORIGIN (\S+)')
def cmd_import(args):
    text = []
    for line in args.file:
        line = re_dos.sub('\n', line)
        text.append(line)
    text = ''.join(text)
    
    m = re_origin.search(text)
    if not m:
        raise Exception, 'Could not find origin'
    origin = m.group(1)
    
    zone = dns.zone.from_text(text, origin=origin, check_origin=True)
    f = BindToR53Formatter()
    xml = f.create_all(zone, exclude=is_root_soa_or_ns)

    ret = r53.change_rrsets(args.zone, xml)
    pprint(ret.ChangeResourceRecordSetsResponse)
    
re_zone_id = re.compile('^[A-Z0-9]{14}$')
def Zone(zone):
    if re_zone_id.match(zone):
        return zone
    ret = r53.get_all_hosted_zones()
    for hz in ret.ListHostedZonesResponse.HostedZones:
        if hz.Name == zone or hz.Name == zone+'.':
            return hz.Id.replace('/hostedzone/', '')
    raise ArgumentError, 'Zone %s not found' % zone
    
def _get_records(args):
    info = r53.get_hosted_zone(args.zone)
    xml = r53.get_all_rrsets(args.zone)
    f = R53ToBindFormatter()
    return f.convert(info.GetHostedZoneResponse, xml)

def cmd_export(args):
    zone = _get_records(args)
    zone.to_file(sys.stdout)
    
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

    if args.replace and rdataset_old:
        xml = BindToR53Formatter().replace_record(zone, name, rdataset_old, rdataset)
    else:
        xml = BindToR53Formatter().create_record(zone, name, rdataset)
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
            print 'Ambigious record - several resource types for record %s found: %s' % (args.rr, ', '.join(rtypes))
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
            
            xml = BindToR53Formatter().delete_record(zone, name, rdataset)
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
    xml = f.delete_all(zone, exclude=is_root_soa_or_ns)
    ret = r53.change_rrsets(args.zone, xml)
    if args.wait:
	    wait_for_sync(ret)
    else:
        pprint(ret.ChangeResourceRecordSetsResponse)
    
def main():
    connection = boto.route53.Route53Connection()
    parser = argparse.ArgumentParser(description='route53 command line tool')
    subparsers = parser.add_subparsers(help='sub-command help')
	
    supported_rtypes = ('A', 'AAAA', 'CNAME', 'SOA', 'NS', 'MX', 'PTR', 'SPF', 'SRV', 'TXT')
    
    parser_list = subparsers.add_parser('list', help='list hosted zones')
    parser_list.set_defaults(func=cmd_list)
    
    parser_list = subparsers.add_parser('info', help='get details of a hosted zone')
    parser_list.add_argument('zone', type=Zone, help='zone name')
    parser_list.set_defaults(func=cmd_info)
    
    parser_describe = subparsers.add_parser('xml', help='get the rrsets xml of a hosted zone')
    parser_describe.add_argument('zone', type=Zone, help='zone name')
    parser_describe.set_defaults(func=cmd_xml)
    
    parser_describe = subparsers.add_parser('export', help='export dns in bind format')
    parser_describe.add_argument('zone', type=Zone, help='zone name')
    parser_describe.set_defaults(func=cmd_export)
    
    parser_describe = subparsers.add_parser('import', help='import dns in bind format')
    parser_describe.add_argument('zone', type=Zone, help='zone name')
    parser_describe.add_argument('-f', '--file', type=argparse.FileType('r'), help='bind file')
    parser_describe.add_argument('--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: false)')
    parser_describe.set_defaults(func=cmd_import)
    
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
    
    args = parser.parse_args()
    args.func(args)
    
if __name__=='__main__':
    main()
