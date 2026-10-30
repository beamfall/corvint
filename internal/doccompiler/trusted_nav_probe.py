# Fixed probe for the explicitly trusted project profile; never a capsule authority.
import hashlib, importlib.metadata, json, os, pathlib, sys

def digest(data):
    return hashlib.sha256(data).hexdigest()

def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode()

def require(ok, message):
    if not ok:
        raise ValueError(message)

def inventory():
    root = pathlib.Path(sys.prefix).resolve()
    rows, count, size = [], 0, 0
    for dist in importlib.metadata.distributions():
        name = dist.metadata['Name']
        require(name and dist.files is not None, 'distribution-files-required')
        files = []
        for entry in sorted(dist.files, key=str):
            if str(entry).endswith('.pyc'):
                continue
            location = pathlib.Path(os.path.abspath(dist.locate_file(entry)))
            require(not location.is_symlink(), 'distribution-symlink')
            resolved = location.resolve()
            require(resolved.is_relative_to(root) and resolved.is_file(), 'distribution-outside-environment')
            current = location
            while current != pathlib.Path(os.path.abspath(sys.prefix)) and current != current.parent:
                require(not current.is_symlink(), 'distribution-symlink')
                current = current.parent
            require(location.stat().st_size <= 134217728 - size, "distribution-limit")
            data = location.read_bytes()
            count += 1
            size += len(data)
            require(count <= 20000 and size <= 134217728, 'distribution-limit')
            if location.name == 'direct_url.json':
                require(not json.loads(data).get('dir_info', {}).get('editable'), 'editable-distribution')
            files.append([str(resolved.relative_to(root)), digest(data)])
        rows.append([name.lower().replace('_', '-'), dist.version, files])
    rows.sort()
    require(len({row[0] for row in rows}) == len(rows), 'duplicate-distribution')
    require(any(row[:2] == ['mkdocs', '1.6.1'] for row in rows), 'mkdocs-version')
    return digest(encoded({'python': sys.version, 'distributions': rows}))

def relative(value):
    require(isinstance(value, str) and value and all(0x21 <= ord(c) <= 0x7e for c in value), 'invalid-path')
    require('\\' not in value and ':' not in value and not value.startswith('/') and all(p not in ('', '.', '..', '.git') for p in value.split('/')), 'invalid-path')
    return value

def run(request):
    observed = inventory()
    if request.get('mode') == 'inventory':
        return {'inventory_sha256': observed}
    require(observed == request['inventory_sha256'], 'inventory-pin-mismatch')
    # Package bytes are checked before importing any third-party loader.
    import yaml
    from mkdocs.config import load_config
    from mkdocs.utils.yaml import get_yaml_loader
    root = pathlib.Path(request['root']).resolve()
    config = root / request['config']
    raw = config.read_bytes()
    require(len(raw) <= 1048576 and digest(raw) == request['config_sha256'], 'config-pin-mismatch')
    text = raw.decode('utf-8')
    loader = get_yaml_loader()
    tokens = list(yaml.scan(text, Loader=loader))
    require(not any(isinstance(t, (yaml.tokens.AliasToken, yaml.tokens.AnchorToken, yaml.tokens.TagToken)) for t in tokens), 'yaml-authority-ambiguous')
    node = yaml.compose(text, Loader=loader)
    require(isinstance(node, yaml.MappingNode), 'config-mapping-required')
    def check(n, depth=0):
        require(depth <= 32, 'yaml-depth')
        if isinstance(n, yaml.MappingNode):
            names = []
            for k, v in n.value:
                require(isinstance(k, yaml.ScalarNode) and k.tag == 'tag:yaml.org,2002:str' and k.value != '<<', 'yaml-key')
                names.append(k.value)
                check(v, depth + 1)
            require(len(set(names)) == len(names), 'duplicate-yaml-key')
        elif isinstance(n, yaml.SequenceNode):
            for v in n.value:
                check(v, depth + 1)
        else:
            require(n.tag in ('tag:yaml.org,2002:str', 'tag:yaml.org,2002:bool', 'tag:yaml.org,2002:null'), 'yaml-value')
    check(node)
    values = yaml.load(text, Loader=loader)
    allowed = {'site_name','nav','docs_dir','site_dir','site_url','use_directory_urls','theme','plugins','markdown_extensions'}
    require(set(values) <= allowed and 'nav' in values, 'unsupported-config-surface')
    require(values.get('theme', 'mkdocs') == 'mkdocs', 'custom-theme')
    require(values.get('plugins', []) == [] and values.get('markdown_extensions', []) == [], 'executable-config')
    docs = relative(values.get('docs_dir', 'docs'))
    docs_root = root / docs
    require(docs_root.is_dir() and docs_root.resolve().is_relative_to(root), 'docs-dir-escape')
    nav_node = next(v for k, v in node.value if k.value == 'nav')
    require(isinstance(nav_node, yaml.SequenceNode), 'nav-sequence-required')
    paths = []
    def nav_paths(nav, depth=0):
        require(depth < 16 and isinstance(nav, list) and 0 < len(nav) <= 64, 'nav-shape')
        for item in nav:
            if isinstance(item, str):
                path = relative(item)
                require(path.endswith('.md'), 'nav-page-type')
                paths.append(path)
            else:
                require(isinstance(item, dict) and len(item) == 1, 'nav-item')
                title, target = next(iter(item.items()))
                require(isinstance(title, str) and 0 < len(title) <= 256 and not any(ord(c) < 32 for c in title), 'nav-title')
                nav_paths([target] if isinstance(target, str) else target, depth + 1)
    nav_paths(values['nav'])
    for path in paths:
        require((docs_root / path).is_file() and (docs_root / path).resolve().is_relative_to(docs_root), 'nav-page-missing')
    # Parser block end marks can include following comments; use the last nonempty
    # token within the proven node so comments/whitespace outside its value survive.
    start_char = nav_node.start_mark.index
    end_char = max(t.end_mark.index for t in tokens if start_char <= t.start_mark.index < nav_node.end_mark.index and t.end_mark.index > t.start_mark.index and t.end_mark.index <= nav_node.end_mark.index)
    start = len(text[:start_char].encode('utf-8'))
    end = len(text[:end_char].encode('utf-8'))
    cfg = load_config(str(config), plugins=[], markdown_extensions=[], site_dir=request['site_dir'], strict=True)
    require(pathlib.Path(cfg.docs_dir).resolve() == docs_root.resolve(), 'effective-docs-dir-mismatch')
    require(cfg.nav == values['nav'], 'nav-authority-mismatch')
    require(config.read_bytes() == raw, 'config-changed')
    require(inventory() == observed, 'inventory-changed')
    return {'owner': request['config'], 'config_sha256': digest(raw), 'docs_dir': docs, 'nav': cfg.nav, 'nav_sha256': digest(encoded(cfg.nav)), 'start_byte': start, 'end_byte': end, 'span_sha256': digest(raw[start:end]), 'inventory_sha256': observed, 'site_dir_override': 'isolated-output'}

try:
    with open(sys.argv[1], 'rb') as stream:
        request = json.load(stream)
    result = run(request)
    sys.stdout.buffer.write(encoded(result))
except Exception as error:
    # Paths and package/config exception text may contain caller secrets.
    sys.stderr.write('trusted-nav-probe-refused:' + type(error).__name__ + '\n')
    sys.exit(2)
