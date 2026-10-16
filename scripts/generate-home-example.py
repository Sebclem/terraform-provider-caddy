#!/usr/bin/env python3
"""Generate typed HCL site literals from the sanitized resolved manifest."""
import json,pathlib,yaml
root=pathlib.Path(__file__).resolve().parents[1]
sites=yaml.safe_load((root/'testdata/legacy/manifest.yaml').read_text())['sites']
def hcl(v):
 if isinstance(v,bool):return str(v).lower()
 if isinstance(v,str):return json.dumps(v)
 if isinstance(v,int):return str(v)
 if isinstance(v,list):return '['+', '.join(hcl(x) for x in v)+']'
 return '{ '+', '.join(json.dumps(k)+' = '+hcl(x) for k,x in v.items())+' }'
text='// Resolved from the sanitized loader fixture. No YAML or raw JSON is consumed by the provider.\nlocals {\n sites = {\n'
for s in sites:
 proxy=s.get('upstream_config') or [];skip=any(isinstance(x,dict) and 'transport http' in x for x in proxy)
 obj={k:s[k] for k in ['upstream','protected','whitelist_local']}
 obj.update(certificate_group='claptrapbot' if 'claptrapbot' in s['tls_cert_dir'] else 'sebclem',health_uri=s.get('health_uri') or '',tls=skip,override_host=any(isinstance(x,str) and x.startswith('header_up Host') for x in proxy),buffered=any(isinstance(x,str) and x.startswith('request_buffers') for x in proxy))
 obj.update(webdav=s['site']=='grafana-webdav.sebclem.fr',jellyfin=s['site']=='jellyfin.sebclem.fr',basic_auth=s['site']=='vmagent.sebclem.fr')
 text+='  '+hcl(s['site'])+' = '+hcl(obj)+'\n'
text+=' }\n local_ips = ["10.0.0.0/8", "168.119.68.214/32", "2001:470:1f13:b58::/64", "2001:861:5286:d5e0::/60"]\n}\n'
(root/'examples/home/sites.tf').write_text(text)
