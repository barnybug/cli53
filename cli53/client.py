import sys
import re
import itertools
import os
from cStringIO import StringIO
import time
import logging

try:
    import boto.route53
    import boto.jsonresponse
    import boto.exception
    import boto.ec2
except ImportError, ex:
    print "Please install latest boto:"
    print "pip install boto"
    print "(error was: %s)" % ex
    sys.exit(-1)

# check boto version
m = re.match('(\d+)\.(\d+)(?:\.(\d+)|b\d+)', boto.__version__)
if m:
    major, minor, other = m.groups()
    major = int(major)
    minor = int(minor)
    if major < 2 or minor < 1:
        print "Please update boto %s >= 2.1.0" % boto.__version__
        print "pip install --upgrade boto"
        sys.exit(-1)

import argparse
from argparse import ArgumentTypeError
from types import StringTypes
try:
    import xml.etree.ElementTree as et
    assert et  # silence warning
except ImportError:
    import elementtree.ElementTree as et

try:
    import dns.zone
    import dns.rdataset
    import dns.node
    import dns.rdtypes
    import dns.rdataclass
    import dns.rdtypes.ANY.SOA
    import dns.rdtypes.ANY.SPF
    import dns.rdtypes.ANY.TXT
    import dns.rdtypes.IN.A
    import dns.rdtypes.IN.AAAA
    import dns.rdtypes.mxbase
    import dns.rdtypes.nsbase
except ImportError:
    print "Please install dnspython:"
    print "pip install dnspython"
    sys.exit(-1)


class ParseException(Exception):
    pass

# Custom base class to prevent changing values
class CustomBase(dns.rdtypes.nsbase.NSBase):
    def to_text(self, **kw):
        return self.target


class AWS:
    RDCLASS = 127

    class _MULTICLASS(int):
        def __eq__(self, x):
            # this is a hack so dns-python is happy mixing AWS and IN classes
            return x == AWS.RDCLASS or x == dns.rdataclass.IN

        def __ne__(self, x):
            return not self == x
    MULTICLASS = _MULTICLASS(RDCLASS)
    dns.rdataclass._by_text['AWS'] = MULTICLASS
    dns.rdataclass._by_value[RDCLASS] = 'AWS'

    class A(dns.rdtypes.IN.A.A):
        def __init__(self, rdclass, rdtype, address, weight,
                     identifier, region, failover):
            super(dns.rdtypes.IN.A.A, self).__init__(rdclass, rdtype)
            self.address = address
            self.weight = weight
            self.identifier = identifier
            self.region = region
            self.failover = failover

        def to_text(self, **kw):
            if kw.get('relativize'):
                if self.weight is not None:
                    return '%s %s %s' % (
                        self.weight, self.address, self.identifier)
                elif self.region is not None:
                    return 'region:%s %s %s' % (
                        self.region, self.address, self.identifier)
                elif self.failover is not None:
                    return 'failover:%s %s %s' % (self.failover,
                        self.address, self.identifier)
            return self.address

        @classmethod
        def from_text(cls, rdclass, rdtype, tok, origin, relativize):
            fst = tok.get_string()
            weight = None
            region = None
            failover = None
            if fst.startswith('region:'):
                region = fst[7:]
            elif fst.startswith('failover:'):
                failover = fst[9:]
            else:
                weight = fst
            address = tok.get_identifier()
            identifier = tok.get_string()
            return cls(rdclass, rdtype, address, weight, identifier, region, failover)

    class CNAME(CustomBase):
        def __init__(self, rdclass, rdtype, target, weight,
                     identifier, region, failover):
            super(CustomBase, self).__init__(rdclass, rdtype, target)
            self.weight = weight
            self.identifier = identifier
            self.region = region
            self.failover = failover

        def to_text(self, **kw):
            if kw.get('relativize'):
                if self.weight is not None:
                    return '%s %s %s' % (
                        self.weight, self.target, self.identifier)
                elif self.region is not None:
                    return 'region:%s %s %s' % (
                        self.region, self.target, self.identifier)
                elif self.failover is not None:
                    return 'failover:%s %s %s' % (self.failover,
                        self.address, self.identifier)
            return self.target

        @classmethod
        def from_text(cls, rdclass, rdtype, tok, origin, relativize):
            fst = tok.get_string()
            weight = None
            region = None
            failover = None
            if fst.startswith('region:'):
                region = fst[7:]
            elif fst.startswith('failover:'):
                failover = fst[9:]
            else:
                weight = fst
            target = tok.get_string()
            identifier = tok.get_string()
            return cls(rdclass, rdtype, target, weight, identifier, region, failover)

    class ALIAS(dns.rdata.Rdata):
        def __init__(self, rdclass, rdtype, hosted_zone_id, dns_name,
                     weight, identifier, region, failover):
            super(AWS.ALIAS, self).__init__(rdclass, rdtype)
            self.alias_hosted_zone_id = hosted_zone_id
            self.alias_dns_name = dns_name
            self.weight = weight
            self.identifier = identifier
            self.region = region
            self.failover = failover

        def to_text(self, **kw):
            if kw.get('relativize'):
                if self.weight is not None:
                    return '%s %s %s %s' % (
                        self.weight,
                        self.alias_hosted_zone_id, self.alias_dns_name,
                        self.identifier)
                elif self.region is not None:
                    return 'region:%s %s %s %s' % (
                        self.region,
                        self.alias_hosted_zone_id, self.alias_dns_name,
                        self.identifier)
                elif self.failover is not None:
                    return 'failover:%s %s %s %s' % (self.failover,
                        self.alias_hosted_zone_id, self.alias_dns_name,
                        self.identifier)
            return '%s %s' % (self.alias_hosted_zone_id, self.alias_dns_name)

        @classmethod
        def from_text(cls, rdclass, rdtype, tok, origin, relativize):
            weight_pattern = re.compile("[0-9]{1,3}$")
            fst = tok.get_string()
            weight = None
            region = None
            identifier = None
            failover = None
            if fst.startswith('region:'):
                region = fst[7:]
                hosted_zone_id = tok.get_identifier()
            elif fst.startswith('failover:'):
                failover = fst[9:]
                hosted_zone_id = tok.get_identifier()
            elif re.match(weight_pattern, fst):
                weight = int(fst)
                hosted_zone_id = tok.get_identifier()
            else:
                hosted_zone_id = fst
            dns_name = tok.get_identifier()
            if region or weight or failover:
                identifier = tok.get_string()
            tok.get_eol()
            return cls(
                rdclass, rdtype, hosted_zone_id, dns_name, weight,
                identifier, region, failover)

    RDTYPE_ALIAS = 65535
    dns.rdatatype._by_text['ALIAS'] = RDTYPE_ALIAS
    dns.rdatatype._by_value[RDTYPE_ALIAS] = 'ALIAS'

# hook this into the parsing
dns.rdata._rdata_modules[(AWS.RDCLASS, dns.rdatatype.A)] = AWS
dns.rdata._rdata_modules[(AWS.RDCLASS, dns.rdatatype.CNAME)] = AWS
dns.rdata._rdata_modules[(AWS.RDCLASS, AWS.RDTYPE_ALIAS)] = AWS


# Custom MX class to prevent changing values
class MX(dns.rdtypes.mxbase.MXBase):
    def to_text(self, **kw):
        return '%d %s' % (self.preference, self.exchange)


class AAAA(dns.rdtypes.IN.AAAA.AAAA):
    pass


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


def pprint(obj, findent='', indent=''):
    if isinstance(obj, basestring):
        logging.info('%s%s' % (findent, obj))
    elif isinstance(obj, boto.jsonresponse.Element):
        i = findent
        for k, v in obj.iteritems():
            if k in ('IsTruncated', 'MaxItems'):
                continue
            if isinstance(v, StringTypes):
                logging.info('%s%s: %s' % (i, k, v))
            else:
                logging.info('%s%s:' % (i, k))
                pprint(v, indent + '  ', indent + '  ')
            i = indent
    elif isinstance(obj, (boto.jsonresponse.ListElement, list)):
        i = findent
        for v in obj:
            pprint(v, i + '- ', i + '  ')
            i = indent
    else:
        raise ValueError('Cannot pprint type %s' % type(obj))


def cmd_list(args, r53):
    ret = retry(r53.get_all_hosted_zones)
    pprint(ret.ListHostedZonesResponse)


def cmd_info(args, r53):
    ret = retry(r53.get_hosted_zone, args.zone)
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
        origin = zone.origin

        creates = self._build_list(zone, exclude)
        deletes = []
        if old_zone:
            deletes = self._build_list(old_zone, exclude)

        createshash = {}
        for c in creates:
            k = (c[0], c[1].rdtype, tuple(sorted(c[1].to_text(origin=origin, relativize=False).split('\n'))))
            createshash[k] = c

        newdeletes = []
        for c in deletes:
            k = (c[0], c[1].rdtype, tuple(sorted(c[1].to_text(origin=origin, relativize=False).split('\n'))))
            if k in createshash:
                del createshash[k]
            else:
                newdeletes.append(c)

        creates = createshash.values()
        deletes = newdeletes
        return self._xml_changes(zone, creates=creates, deletes=deletes)

    def delete_all(self, zone, exclude=None):
        return self._xml_changes(zone, deletes=self._build_list(zone, exclude))

    def create_record(self, zone, name, rdataset):
        return self._xml_changes(zone, creates=[(name, rdataset)])

    def delete_record(self, zone, name, rdataset):
        return self._xml_changes(zone, deletes=[(name, rdataset)])

    def replace_record(self, zone, name, rdataset_old, rdataset_new):
        return self._xml_changes(
            zone, creates=[(name, rdataset_new)],
            deletes=[(name, rdataset_old)])

    def replace_records(self, zone, creates=None, deletes=None):
        return self._xml_changes(zone, creates=creates, deletes=deletes)

    def _xml_changes(self, zone, creates=None, deletes=None):
        for page in paginate(self._iter_changes(creates, deletes), 100):
            yield self._batch_change(zone, page)

    def _iter_changes(self, creates, deletes):
        for chg, rdatasets in (
                ('DELETE', deletes or []),
                ('CREATE', creates or [])):
            for name, rdataset in rdatasets:
                yield chg, name, rdataset

    def _batch_change(self, zone, chgs):
        root = et.Element(
            'ChangeResourceRecordSetsRequest',
            xmlns=boto.route53.Route53Connection.XMLNameSpace)
        change_batch = et.SubElement(root, 'ChangeBatch')
        changes = et.SubElement(change_batch, 'Changes')

        for chg, name, rdataset in chgs:

            if rdataset.rdclass == AWS.RDCLASS:
                if rdataset.rdtype == AWS.RDTYPE_ALIAS:
                    for rdtype in rdataset.items:
                        rrset = self._change(changes, chg, zone, name)
                        evaluateTargetHealth = False
                        text_element(rrset, 'Type', 'A')
                        if rdtype.weight:
                            text_element(
                                rrset, 'SetIdentifier',
                                rdtype.identifier)
                            text_element(rrset, 'Weight', str(rdtype.weight))
                        elif rdtype.region:
                            text_element(
                                rrset, 'SetIdentifier',
                                rdtype.identifier)
                            text_element(rrset, 'Region', str(rdtype.region))
                        elif rdtype.failover:
                            text_element(
                                rrset, 'SetIdentifier',
                                rdtype.identifier)
                            text_element(rrset, 'Failover', str(rdtype.failover))
                            evaluateTargetHealth = (rdtype.failover == 'PRIMARY')
                        at = et.SubElement(rrset, 'AliasTarget')
                        text_element(
                            at, 'HostedZoneId',
                            rdtype.alias_hosted_zone_id)
                        text_element(at, 'DNSName', rdtype.alias_dns_name)
                        text_element(at, 'EvaluateTargetHealth', str(evaluateTargetHealth).lower())
                elif rdataset.rdtype in (dns.rdatatype.A, dns.rdatatype.CNAME):
                    # Weighted A expands into multiple records (as each can
                    # have its own weighting/identifier)
                    for rdtype in rdataset.items:
                        rrset = self._change(changes, chg, zone, name)
                        text_element(
                            rrset, 'Type',
                            dns.rdatatype.to_text(rdataset.rdtype))
                        text_element(rrset, 'SetIdentifier', rdtype.identifier)
                        if rdtype.weight is not None:
                            text_element(rrset, 'Weight', str(rdtype.weight))
                        elif rdtype.region:
                            text_element(rrset, 'Region', str(rdtype.region))
                        elif rdtype.failover:
                            text_element(rrset, 'Failover', str(rdtype.failover))
                        text_element(rrset, 'TTL', str(rdataset.ttl))
                        rrs = et.SubElement(rrset, 'ResourceRecords')
                        rr = et.SubElement(rrs, 'ResourceRecord')
                        text_element(
                            rr, 'Value',
                            rdtype.to_text(
                                origin=zone.origin,
                                relativize=False))
            else:
                rrset = self._change(changes, chg, zone, name)
                text_element(
                    rrset, 'Type',
                    dns.rdatatype.to_text(rdataset.rdtype))
                text_element(rrset, 'TTL', str(rdataset.ttl))
                rrs = et.SubElement(rrset, 'ResourceRecords')
                for rdtype in rdataset.items:
                    rr = et.SubElement(rrs, 'ResourceRecord')
                    text_element(
                        rr, 'Value',
                        rdtype.to_text(origin=zone.origin, relativize=False))

        out = StringIO()
        et.ElementTree(root).write(out)
        return out.getvalue()

    def _change(self, changes, chg, zone, name):
        change = et.SubElement(changes, 'Change')
        text_element(change, 'Action', chg)
        rrset = et.SubElement(change, 'ResourceRecordSet')
        text_element(rrset, 'Name', name.derelativize(zone.origin).to_text())
        return rrset


class R53ToBindFormatter(object):
    def get_all_rrsets(self, r53, ghz, zone):
        rrsets = retry(r53.get_all_rrsets, zone)
        return retry(self.convert, ghz, rrsets)

    def convert(self, info, rrsets, z=None):
        if not z:
            origin = info.HostedZone.Name
            origin = origin.replace('\\057', '/')
            z = dns.zone.Zone(dns.name.from_text(origin))

        for rrset in rrsets:
            name = rrset.name
            if '\\052' in name:
                # * char seems to confuse Amazon and is returned as \\052
                name = name.replace('\\052', '*')
            name = name.replace('\\057', '/')
            rtype = rrset.type
            ttl = int(rrset.ttl)
            values = rrset.resource_records

            if rrset.alias_dns_name is not None:
                rtype = 'ALIAS'
                values = ['%s %s' % (
                    rrset.alias_hosted_zone_id,
                    rrset.alias_dns_name)]

            rdataset = _create_rdataset(
                rtype, ttl, values, rrset.weight,
                rrset.identifier, getattr(rrset, 'region', None), getattr(rrset, 'failover', None))
            node = z.get_node(name, create=True)
            node.rdatasets.append(rdataset)

        return z

re_quoted = re.compile(r'^".*"$')
re_quotepair = re.compile(r'"\s*"')
re_backslash = re.compile(r'\\(.)')


def unquote_list(v):
    if not re_quoted.match(v):
        return [v]
    return [re_backslash.sub('\\1', s) for s in re_quotepair.split(v[1:-1])]


def _create_rdataset(rtype, ttl, values, weight, identifier, region, failover):
    rdataset = dns.rdataset.Rdataset(
        dns.rdataclass.IN,
        dns.rdatatype.from_text(rtype))
    rdataset.ttl = ttl
    for value in values:
        rdtype = None
        if rtype == 'A':
            if identifier is None:
                rdtype = dns.rdtypes.IN.A.A(
                    dns.rdataclass.IN, dns.rdatatype.A,
                    value)
            else:
                rdataset.rdclass = AWS.RDCLASS
                if weight is not None:
                    rdtype = AWS.A(
                        AWS.RDCLASS, dns.rdatatype.A, value, weight,
                        identifier, None, None)
                elif region is not None:
                    rdtype = AWS.A(
                        AWS.RDCLASS, dns.rdatatype.A, value, None,
                        identifier, region, None)
                elif failover is not None:
                    rdtype = AWS.A(AWS.RDCLASS, dns.rdatatype.A, value, None,
                        identifier, None, failover)
        elif rtype == 'AAAA':
            rdtype = AAAA(dns.rdataclass.IN, dns.rdatatype.AAAA, value)
        elif rtype == 'CNAME':
            if identifier is None:
                rdtype = CNAME(dns.rdataclass.ANY, dns.rdatatype.CNAME, value)
            else:
                rdataset.rdclass = AWS.RDCLASS
                rdtype = AWS.CNAME(
                    AWS.RDCLASS, dns.rdatatype.CNAME, value,
                    weight, identifier, region, failover)
        elif rtype == 'SOA':
            mname, rname, serial, refresh, retry, expire, minimum = value.split()
            mname = dns.name.from_text(mname)
            rname = dns.name.from_text(rname)
            serial = int(serial)
            refresh = int(refresh)
            retry = int(retry)
            expire = int(expire)
            minimum = int(minimum)
            rdtype = dns.rdtypes.ANY.SOA.SOA(
                dns.rdataclass.ANY,
                dns.rdatatype.SOA, mname, rname, serial, refresh, retry,
                expire, minimum)
        elif rtype == 'NS':
            rdtype = NS(dns.rdataclass.ANY, dns.rdatatype.NS, value)
        elif rtype == 'MX':
            try:
                pref, ex = value.split()
                pref = int(pref)
            except ValueError:
                raise ValueError('mx records require two parts: priority name')
            rdtype = MX(dns.rdataclass.ANY, dns.rdatatype.MX, pref, ex)
        elif rtype == 'PTR':
            rdtype = PTR(dns.rdataclass.ANY, dns.rdatatype.PTR, value)
        elif rtype == 'SPF':
            rdtype = SPF(dns.rdataclass.ANY, dns.rdatatype.SPF, value)
        elif rtype == 'SRV':
            rdtype = SRV(dns.rdataclass.IN, dns.rdatatype.SRV, value)
        elif rtype == 'TXT':
            values = unquote_list(value)
            rdtype = dns.rdtypes.ANY.TXT.TXT(
                dns.rdataclass.ANY,
                dns.rdatatype.TXT, values)
        elif rtype == 'ALIAS':
            rdataset.rdclass = AWS.RDCLASS
            try:
                hosted_zone_id, dns_name = value.split()
            except ValueError:
                raise ValueError('ALIAS records require two parts: hosted zone id and dns name of ELB')
            if identifier is None:
                rdtype = AWS.ALIAS(AWS.RDCLASS, AWS.RDTYPE_ALIAS, hosted_zone_id, dns_name, None, identifier, None, None)
            else:
                if weight is not None:
                    rdtype = AWS.ALIAS(
                        AWS.RDCLASS, AWS.RDTYPE_ALIAS, hosted_zone_id, dns_name, weight, identifier, None, None)
                elif region is not None:
                    rdtype = AWS.ALIAS(
                        AWS.RDCLASS, AWS.RDTYPE_ALIAS, hosted_zone_id, dns_name, None, identifier, region, None)
                elif failover is not None:
                    rdtype = AWS.ALIAS(
                        AWS.RDCLASS, AWS.RDTYPE_ALIAS, hosted_zone_id, dns_name, None, identifier, None, failover)
                else:
                    raise ValueError('unsupported alias type')
        else:
            raise ValueError('record type %s not handled' % rtype)
        rdataset.items.append(rdtype)
    return rdataset

re_dos = re.compile('\r\n$')
re_origin = re.compile(r'\$ORIGIN[ \t](\S+)')
re_include = re.compile(r'\$INCLUDE[ \t](\S+)')
def cmd_import(args, r53):
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
        raise ParseException('%s: Could not find $ORIGIN' % args.file.name)
    origin = m.group(1)
    if not origin.endswith('.'):
        raise ParseException('%s: $ORIGIN must end with a period (.)' % args.file.name)

    try:
        zone = dns.zone.from_text(text, origin=origin, check_origin=False)
    except dns.exception.SyntaxError as ex:
        raise ParseException('%s: %s' % (args.file.name, ex))

    old_zone = None
    if args.replace:
        old_zone = _get_records(args, r53)

    f = BindToR53Formatter()

    if args.editauth:
        exclude_rr = None
    else:
        exclude_rr = is_root_soa_or_ns

    for xml in f.create_all(zone, old_zone=old_zone, exclude=exclude_rr):
        if args.dump:
            logging.debug(xml)
        ret = retry(r53.change_rrsets, args.zone, xml)
        if args.wait:
            wait_for_sync(ret, r53)
        else:
            pprint(ret.ChangeResourceRecordSetsResponse)

re_zone_id = re.compile('^[A-Z0-9]+$')
def ZoneFactory(r53):
    def Zone(zone):
        if re_zone_id.match(zone):
            return zone
        ret = retry(r53.get_all_hosted_zones)

        zone = zone.replace('/', '\\057')
        hzs = [
            hz.Id.replace('/hostedzone/', '')
            for hz in ret.ListHostedZonesResponse.HostedZones if hz.Name == zone or hz.Name == zone + '.'
        ]
        if len(hzs) == 1:
            return hzs[0]
        elif len(hzs) > 1:
            raise ArgumentTypeError(
                'Zone %r is ambiguous (matches: %s), please specify zone as ID' % (zone, ', '.join(hzs)))
        else:
            raise ArgumentTypeError('Zone %r not found' % zone)
    return Zone

def _get_records(args, r53):
    info = retry(r53.get_hosted_zone, args.zone)
    f = R53ToBindFormatter()
    return f.get_all_rrsets(r53, info.GetHostedZoneResponse, args.zone)

def cmd_export(args, r53):
    zone = _get_records(args, r53)
    print '$ORIGIN %s' % zone.origin.to_text()
    zone.to_file(sys.stdout, relativize=not args.full)

def _read_aws_cfg(filename):
    import ConfigParser
    config = ConfigParser.RawConfigParser()
    config.read(filename)
    for section in config.sections():
        if section.startswith('profile '):
            logging.debug('Scanning account: %s' % section)
            aws_access_key_id = config.get(section, 'aws_access_key_id')
            aws_secret_access_key = config.get(section, 'aws_secret_access_key')
            region = config.get(section, 'region')

            try:
                yield boto.ec2.connect_to_region(
                    region,
                    aws_access_key_id=aws_access_key_id,
                    aws_secret_access_key=aws_secret_access_key)
            except:
                logging.exception('Failed connecting to account: %s' % section)

def cmd_instances(args, r53):
    logging.info('Getting DNS records')
    zone = _get_records(args, r53)
    if args.off:
        filters = {}
    else:
        filters = {'instance-state-name': 'running'}

    if args.credentials:
        connections = _read_aws_cfg(args.credentials)
    else:
        connections = [boto.ec2.connect_to_region(region) for region in args.regions.split(',')]

    def get_instances():
        for conn in connections:
            for r in conn.get_all_instances(filters=filters):
                for i in r.instances:
                    yield i

    suffix = '.' + zone.origin.to_text().strip('.')
    creates = []
    deletes = []
    instances = get_instances()
    # limit to instances with a Name tag
    instances = (i for i in instances if i.tags.get('Name'))
    if args.match:
        instances = (i for i in instances if re.search(args.match, i.tags['Name']))
    logging.info('Getting EC2 instances')
    instances_by_name = {}
    for inst in instances:
        name = inst.tags.get('Name')
        if not name:
            continue

        # strip domain suffix if present
        if name.endswith(suffix):
            name = name[0:-len(suffix)]
        name = dns.name.from_text(name, zone.origin)

        if name not in instances_by_name or inst.state == 'running':
            # on duplicate named instances, running takes priority
            instances_by_name[name] = inst

    if args.write_a_record:
        rtype = dns.rdatatype.A
    else:
        rtype = dns.rdatatype.CNAME

    for name, inst in instances_by_name.iteritems():
        node = zone.get_node(name)
        if node and node.rdatasets and node.rdatasets[0].rdtype != rtype:
            # don't replace/update existing manually created records
            logging.warning("Not overwriting record for %s as it appears to have been manually created" % name)
            continue

        newvalue = None
        if inst.state == 'running':
            if inst.public_dns_name and not args.internal:
                newvalue = inst.ip_address if args.write_a_record else inst.public_dns_name
            else:
                newvalue = inst.private_ip_address if args.write_a_record else inst.private_dns_name
        elif args.off == 'delete':
            newvalue = None
        elif args.off and name not in creates:
            newvalue = args.off

        if node:
            if args.write_a_record:
                oldvalue = node.rdatasets[0].items[0].address
            else:
                oldvalue = node.rdatasets[0].items[0].target.strip('.')
            if oldvalue != newvalue:
                if newvalue:
                    logging.info('Updating record for %s: %s -> %s' % (name, oldvalue, newvalue))
                else:
                    logging.info('Deleting record for %s: %s' % (name, oldvalue))
                deletes.append((name, node.rdatasets[0]))
            else:
                logging.debug('Record %s unchanged' % name)
                continue
        else:
            logging.info('Creating record for %s: %s' % (name, newvalue))

        if newvalue:
            if args.write_a_record:
                rd = _create_rdataset('A', args.ttl, [newvalue], None, None, None, None)
            else:
                rd = _create_rdataset('CNAME', args.ttl, [newvalue], None, None, None, None)
            creates.append((name, rd))

    if not deletes and not creates:
        logging.info('No changes')
        return

    if args.dry_run:
        logging.info('Dry run - not making changes')
        return

    f = BindToR53Formatter()
    parts = f.replace_records(zone, creates, deletes)
    for xml in parts:
        ret = retry(r53.change_rrsets, args.zone, xml)
        if args.wait:
            wait_for_sync(ret, r53)
        else:
            logging.info('Success')
            pprint(ret.ChangeResourceRecordSetsResponse)

def cmd_create(args, r53):
    ret = retry(r53.create_hosted_zone, args.zone, comment=args.comment)
    if args.wait:
        wait_for_sync(ret, r53)
    else:
        pprint(ret.CreateHostedZoneResponse)

def retry(fn, *args, **kwargs):
    sleep_time = 1
    while True:
        try:
            return fn(*args, **kwargs)
        except boto.route53.exception.DNSServerError as e:
            if e.error_code == 'Throttling':
                time.sleep(sleep_time)
                sleep_time *= 2
            else:
                raise

def cmd_delete(args, r53):
    ret = retry(r53.delete_hosted_zone, args.zone)
    if hasattr(ret, 'ErrorResponse'):
        pprint(ret.ErrorResponse)
    elif args.wait:
        wait_for_sync(ret, r53)
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

def wait_for_sync(obj, r53):
    waiting = True
    id = find_key_nonrecursive(obj, 'Id')
    id = id.replace('/change/', '')
    sys.stdout.write("Waiting for change to sync")
    ret = ''
    sleep_time = 1
    while waiting:
        try:
            ret = r53.get_change(id)
            status = find_key_nonrecursive(ret, 'Status')
            if status == 'INSYNC':
                waiting = False
            else:
                sys.stdout.write('.')
                sys.stdout.flush()
                time.sleep(sleep_time)
        except boto.route53.exception.DNSServerError as e:
            if e.error_code == 'Throttling':
                time.sleep(sleep_time)
                sleep_time *= 2
            else:
                raise
    logging.info("Completed")
    pprint(ret.GetChangeResponse)

def cmd_rrcreate(args, r53):
    zone = _get_records(args, r53)
    name = dns.name.from_text(args.rr, zone.origin)
    rdataset = _create_rdataset(args.type, args.ttl, args.values, args.weight, args.identifier, args.region, args.failover)

    rdataset_old = None
    node = zone.get_node(args.rr)
    if node:
        for rds in node.rdatasets:
            if args.type == dns.rdatatype.to_text(rds.rdtype):
                # find the rds in the requested region only
                if args.region is not None:
                    for rdtype in rds.items:
                        if hasattr(rdtype, 'region') and rdtype.region == args.region:
                            rdataset_old = rds
                            break
                else:
                    rdataset_old = rds
                    break

    f = BindToR53Formatter()
    if args.replace and rdataset_old:
        parts = f.replace_record(zone, name, rdataset_old, rdataset)
    else:
        parts = f.create_record(zone, name, rdataset)
    for xml in parts:
        if args.dump:
            logging.debug(xml)
        ret = retry(r53.change_rrsets, args.zone, xml)
        if args.wait:
            wait_for_sync(ret, r53)
        else:
            logging.info('Success')
            pprint(ret.ChangeResourceRecordSetsResponse)

def cmd_rrdelete(args, r53):
    zone = _get_records(args, r53)
    name = dns.name.from_text(args.rr, zone.origin)

    node = zone.get_node(args.rr)
    if node:
        if len(node.rdatasets) > 1 and not args.type:
            rtypes = [dns.rdatatype.to_text(rds.rdtype) for rds in node.rdatasets]
            logging.warning(
                'Ambigious record - several resource types for record %r found: %s' % (
                    args.rr, ', '.join(rtypes)))
        else:
            rdataset = None
            for rds in node.rdatasets:
                if args.type == dns.rdatatype.to_text(rds.rdtype) or not args.type:
                    if args.identifier is not None:
                        for rdtype in rds.items:
                            if hasattr(rdtype, 'identifier') and rdtype.identifier == args.identifier:
                                rdataset = rds
                                break
                    else:
                        rdataset = rds
                        break

            if not rdataset:
                logging.warning('Record not found: %s, type: %s' % (args.rr, args.type))
                return

            logging.info('Deleting %s %s...' % (args.rr, dns.rdatatype.to_text(rds.rdtype)))

            f = BindToR53Formatter()
            for xml in f.delete_record(zone, name, rdataset):
                ret = retry(r53.change_rrsets, args.zone, xml)
                if args.wait:
                    wait_for_sync(ret, r53)
                else:
                    logging.info('Success')
                    pprint(ret.ChangeResourceRecordSetsResponse)
    else:
        logging.warning('Record not found: %s' % args.rr)

def cmd_rrpurge(args, r53):
    zone = _get_records(args, r53)
    f = BindToR53Formatter()
    for xml in f.delete_all(zone, exclude=is_root_soa_or_ns):
        ret = retry(r53.change_rrsets, args.zone, xml)
        if args.wait:
            wait_for_sync(ret, r53)
        else:
            pprint(ret.ChangeResourceRecordSetsResponse)

def cmd_rrlist(args, r53):
    zone = _get_records(args, r53)
    print '\t'.join(["host", "ttl", "cls", "type", "data"])
    for record_name, record_value in zone.iteritems():
        print '\t'.join(record_value.to_text(record_name).split(' '))

def get_route53_connection():
    try:
        return boto.route53.Route53Connection()
    except boto.exception.NoAuthHandlerFound:
        print 'Please configure your AWS credentials, either through environment '\
              'variables or'
        print '~/.boto config file.'
        print 'e.g.'
        print 'export AWS_ACCESS_KEY_ID=XXXXXXXXXXXXXX'
        print 'export AWS_SECRET_ACCESS_KEY=XXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
        print 'or in ~/.boto:'
        print '[Credentials]'
        print 'aws_access_key_id = XXXXXXXXXXXXXX'
        print 'aws_secret_access_key = XXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
        print
        print 'See: http://code.google.com/p/boto/wiki/BotoConfig'
        sys.exit(-1)

def main(connection=None):
    print "WARNING: out of date version of cli53 installed."
    print "cli53 0.5 (python version) is no longer being actively maintained,"
    print "to install the latest version see:"
    print "https://github.com/barnybug/cli53/#installation"
    print ""
    print "You will need to 'pip uninstall cli53' first."

    if not connection:
        connection = get_route53_connection()

    Zone = ZoneFactory(connection)

    parser = argparse.ArgumentParser(description='route53 command line tool')
    parser.add_argument('-d', '--debug', action='store_true', help='Turn on debugging')
    parser.add_argument('--logconfig', help='Specify logging configuration')
    subparsers = parser.add_subparsers(help='sub-command help')

    supported_rtypes = ('A', 'AAAA', 'CNAME', 'SOA', 'NS', 'MX', 'PTR', 'SPF', 'SRV', 'TXT', 'ALIAS')

    parser_list = subparsers.add_parser('list', help='list hosted zones')
    parser_list.set_defaults(func=cmd_list)

    parser_info = subparsers.add_parser('info', help='get details of a hosted zone')
    parser_info.add_argument('zone', type=Zone, help='zone name')
    parser_info.set_defaults(func=cmd_info)

    parser_describe = subparsers.add_parser('export', help='export dns in bind format')
    parser_describe.add_argument('zone', type=Zone, help='zone name')
    parser_describe.add_argument('--full', action='store_true', help='export prefixes as full names')
    parser_describe.set_defaults(func=cmd_export)

    parser_import = subparsers.add_parser('import', help='import dns in bind format')
    parser_import.add_argument('zone', type=Zone, help='zone name')
    parser_import.add_argument(
        '-r', '--replace', action='store_true', help='replace all existing records (use with care!)')
    parser_import.add_argument('-f', '--file', type=argparse.FileType('r'), help='bind file')
    parser_import.add_argument(
        '--wait', action='store_true', default=False,
        help='wait for changes to become live before exiting (default: false)')
    parser_import.add_argument(
        '--editauth', action='store_true', default=False, help='include SOA and NS records from zone file')
    parser_import.add_argument('--dump', action='store_true', help='dump xml format to stdout')
    parser_import.set_defaults(func=cmd_import)

    parser_instances = subparsers.add_parser('instances', help='dynamically update your dns with instance names')
    parser_instances.add_argument('zone', type=Zone, help='zone name')
    parser_instances.add_argument(
        '--off', default=False,
        help='if provided, then records for stopped instances will be updated. If set to "delete", they are removed, '
        'otherwise this option gives the dns name the CNAME should revert to')
    parser_instances.add_argument(
        '--regions', default=os.getenv('EC2_REGION', 'us-east-1'), help='a comma-separated list of regions to check '
        '(default: environment variable EC2_REGION, or otherwise "us-east-1")')
    parser_instances.add_argument(
        '--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: '
        'false)')
    parser_instances.add_argument('-x', '--ttl', type=int, default=60, help='resource record ttl')
    parser_instances.add_argument('--match', help='regular expression to select which Name tags will be qualify')
    parser_instances.add_argument(
        '--credentials', help='separate credentials file containing account(s) to check for instances')
    parser_instances.add_argument(
        '-i', '--internal', action='store_true', default=False, help='always use the internal hostname')
    parser_instances.add_argument(
        '-a', '--write-a-record', action='store_true', default=False, help='write an A record (IP) instead of CNAME')
    parser_instances.add_argument('-n', '--dry-run', action='store_true', help='dry run - don\'t make any changes')
    parser_instances.set_defaults(func=cmd_instances)

    parser_create = subparsers.add_parser('create', help='create a hosted zone')
    parser_create.add_argument('zone', help='zone name')
    parser_create.add_argument(
        '--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: '
        'false)')
    parser_create.add_argument('--comment', help='add a comment to the zone')
    parser_create.set_defaults(func=cmd_create)

    parser_delete = subparsers.add_parser('delete', help='delete a hosted zone')
    parser_delete.add_argument('zone', type=Zone, help='zone name')
    parser_delete.add_argument(
        '--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: '
        'false)')
    parser_delete.set_defaults(func=cmd_delete)

    parser_rrcreate = subparsers.add_parser('rrcreate', help='create a resource record')
    parser_rrcreate.add_argument('zone', type=Zone, help='zone name')
    parser_rrcreate.add_argument('rr', help='resource record')
    parser_rrcreate.add_argument('type', choices=supported_rtypes, help='resource record type')
    parser_rrcreate.add_argument('values', nargs='+', help='resource record values')
    parser_rrcreate.add_argument('-x', '--ttl', type=int, default=86400, help='resource record ttl')
    parser_rrcreate.add_argument('-w', '--weight', type=int, help='record weight')
    parser_rrcreate.add_argument('-i', '--identifier', help='record set identifier')
    parser_rrcreate.add_argument('--region', help='region for latency-based routing')
    parser_rrcreate.add_argument('--failover', choices=['PRIMARY', 'SECONDARY'], help='failover type for dns failover routing')
    parser_rrcreate.add_argument('-r', '--replace', action='store_true', help='replace any existing record')
    parser_rrcreate.add_argument(
        '--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: '
        'false)')
    parser_rrcreate.add_argument('--dump', action='store_true', help='dump xml format to stdout')
    parser_rrcreate.set_defaults(func=cmd_rrcreate)

    parser_rrdelete = subparsers.add_parser('rrdelete', help='delete a resource record')
    parser_rrdelete.add_argument('zone', type=Zone, help='zone name')
    parser_rrdelete.add_argument('rr', help='resource record')
    parser_rrdelete.add_argument('type', nargs='?', choices=supported_rtypes, help='resource record type')
    parser_rrdelete.add_argument('-i', '--identifier', help='record set identifier')
    parser_rrdelete.add_argument(
        '--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: '
        'false)')
    parser_rrdelete.set_defaults(func=cmd_rrdelete)

    parser_rrpurge = subparsers.add_parser('rrpurge', help='purge all resource records')
    parser_rrpurge.add_argument('zone', type=Zone, help='zone name')
    parser_rrpurge.add_argument(
        '--confirm', required=True, action='store_true', help='confirm you definitely want to do this!')
    parser_rrpurge.add_argument(
        '--wait', action='store_true', default=False, help='wait for changes to become live before exiting (default: '
        'false)')
    parser_rrpurge.set_defaults(func=cmd_rrpurge)

    parser_rrlist = subparsers.add_parser('rrlist', help='list all resource records')
    parser_rrlist.add_argument('zone', type=Zone, help='zone name')
    parser_rrlist.set_defaults(func=cmd_rrlist)

    args = parser.parse_args()
    if args.logconfig:
        logging.config.fileConfig(args.logconfig)
    else:
        if args.debug:
            level = logging.DEBUG
        else:
            level = logging.INFO
        logging.basicConfig(
            level=level, format="%(message)s",
            stream=sys.stdout)
        logging.getLogger('boto').setLevel(logging.WARNING)

    try:
        args.func(args, r53=connection)
    except ParseException as ex:
        raise SystemExit("Parse error: %s" % ex)
