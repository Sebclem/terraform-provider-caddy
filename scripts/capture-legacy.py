#!/usr/bin/env python3
"""Read only public YAML; execute the installed loader's site-resolution method.
Requires Python PyYAML/Jinja2. Never loads vaults or runs Ansible tasks.
"""
import ast, hashlib, json, os, pathlib, subprocess, sys, types
import yaml
from jinja2 import Environment, StrictUndefined
from jinja2.nativetypes import NativeEnvironment
repo = pathlib.Path(sys.argv[1])
loader = pathlib.Path(sys.argv[2])
out = pathlib.Path(__file__).resolve().parents[1] / 'testdata/legacy'
source = loader.read_text()
node = next(n for n in ast.walk(ast.parse(source)) if isinstance(n, ast.FunctionDef) and n.name == '_load_files_in_dir')
namespace = {'to_native': str, 'path': os.path, 'Path': pathlib.Path, 'display': types.SimpleNamespace(vvv=lambda _: None)}
exec(compile(ast.Module(body=[node], type_ignores=[]), str(loader), 'exec'), namespace)
vars = {}
for p in ['inventories/group_vars/all/vars.yml', 'inventories/group_vars/home/vars.yml']:
    vars.update(yaml.safe_load((repo / p).read_text()))
hash = subprocess.check_output([os.environ['CADDY_BIN'], 'hash-password', '--plaintext', 'fixture-password'], text=True).strip()
for user in ['vm','grafana','opnsense','admin']:
    vars['vault_userlist_victoria_'+user+'_password'] = hash
native = NativeEnvironment(undefined=StrictUndefined)
def expand(v):
    if isinstance(v, dict): return {k: expand(x) for k,x in v.items()}
    if isinstance(v, list): return [expand(x) for x in v]
    if isinstance(v, str) and '{{' in v: return expand(native.from_string(v).render(**vars))
    return v
obj = types.SimpleNamespace(_task=types.SimpleNamespace(_role=None), default_domain=vars['default_base_domain'], default_dns_provider=vars['default_dns_provider'], default_dns_target='fixture.invalid.', default_tls_cert_dir=vars['caddy_default_tls_cert_dir'])
obj._load_files = lambda p: (False, '', yaml.safe_load(pathlib.Path(p).read_text()))
directory = repo / 'playbooks/config/home'
failed, err, result = namespace['_load_files_in_dir'](obj, str(directory), sorted(p.name for p in directory.glob('*.yml')))
assert not failed, err
sites = expand(result['sites_config'])
vars.update(caddy_sites=sites, caddy_snippet=expand(vars['caddy_snippet']), caddy_named_routes=[])
text = Environment(undefined=StrictUndefined).from_string((repo/'playbooks/templates/Caddyfile.j2').read_text()).render(**vars)
out.mkdir(exist_ok=True)
(out/'active.caddy').write_text(text+'\n')
adapt = subprocess.run([os.environ['CADDY_BIN'],'adapt','--adapter','caddyfile','--config',str(out/'active.caddy')], capture_output=True, text=True, check=True)
(out/'active.json').write_text(json.dumps(json.loads(adapt.stdout),indent=2)+'\n')
(out/'manifest.yaml').write_text(yaml.safe_dump({'loader_sha256':hashlib.sha256(source.encode()).hexdigest(),'sites':sites},sort_keys=False))
print(f'Captured {len(sites)} active sites; no vaults read.')
