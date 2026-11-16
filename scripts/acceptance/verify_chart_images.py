"""Read actual native Save outputs without regenerating or rewriting them."""
import argparse
import hashlib
import json
from pathlib import Path
import xml.etree.ElementTree as ET
from PIL import Image

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--private', type=Path, required=True)
parser.add_argument('--public', type=Path, required=True)
parser.add_argument('--arch', choices=['amd64', '386'], required=True)
args = parser.parse_args()
units = dict(samples='samples', nanoseconds='ns', microseconds='µs', milliseconds='ms', seconds='s', bytes='bytes', unknown='Unknown')
run = args.private / ('chart-amd64' if args.arch == 'amd64' else 'chart-386-fixed')
native_bytes = (run / 'layout.json').read_bytes()
native = json.loads(native_bytes)
assert native['report']['passed'] and native['osScreenScaleFactor'] == native['webviewZoom'] == 1
assert [c['unit'] for c in native['report']['cases']] == list(units)
destination = args.public / ('images-' + args.arch)
destination.mkdir(exist_ok=True)
cases = []
for case in native['report']['cases']:
    unit, axis = case['unit'], units[case['unit']]
    assert case['nativeSaveCompleted'] and case['svgAxisObserved'] and case['canvasObserved']
    assert [(s['renderer'], s['format']) for s in case['saves']] == [('canvas', 'svg'), ('svg', 'png')]
    assert all(s['nativeSaveCompleted'] for s in case['saves'])
    svg_name, png_name = unit + '-canvas.svg', unit + '-svg.png'
    svg_bytes = (args.private / ('images-' + args.arch) / svg_name).read_bytes()
    root = ET.fromstring(svg_bytes)
    assert root.tag == '{http://www.w3.org/2000/svg}svg'
    width, height = int(root.attrib['width']), int(root.attrib['height'])
    texts = [e.text for e in root.iter() if e.tag == '{http://www.w3.org/2000/svg}text']
    assert axis in texts and 'Other' in texts and '1,000,000' in texts
    bars = [e for e in root.iter() if e.tag.endswith('path') and e.attrib.get('fill') == '#0f766e' and e.attrib.get('d')]
    assert len(bars) == 1 and width > 100 and height > 100
    assert [case['saves'][0]['width'], case['saves'][0]['height']] == [width, height]
    # Initial x64 harness selected a toolbar SVG icon for its PNG bounds metadata.
    # Preserve the raw receipt, reject that field as evidence, and use actual files.
    if args.arch == '386':
        assert [case['saves'][1]['width'], case['saves'][1]['height']] == [width, height]
    png_bytes = (args.private / ('images-' + args.arch) / png_name).read_bytes()
    assert png_bytes.startswith(b'\x89PNG\r\n\x1a\n')
    with Image.open(args.private / ('images-' + args.arch) / png_name) as image:
        image.load()
        assert image.format == 'PNG' and image.size == (2*width, 2*height)
        rgb = image.convert('RGB')
        # Real filled bar occupies substantial area; a blank canvas cannot pass.
        teal = sum(count for count,(r,g,b) in rgb.getcolors(image.width * image.height) if r < 50 and 70 < g < 160 and 60 < b < 150)
        assert teal > image.width * image.height * 0.2
        assert rgb.getpixel((0,0)) == (255,255,255)
    for name, data in [(svg_name, svg_bytes), (png_name, png_bytes)]:
        (destination / name).write_bytes(data)
    cases.append(dict(unit=unit, axis=axis, value=1000000, svgSize=[width,height], pngSize=[width*2,height*2],
        svgSha256=hashlib.sha256(svg_bytes).hexdigest(), pngSha256=hashlib.sha256(png_bytes).hexdigest(), passed=True))
(args.public / ('chart-native-' + args.arch + '.json')).write_bytes(native_bytes)
report = dict(passed=True, scope='Seven units; native Windows Canvas→SVG and SVG→PNG Save outputs; no Office/reference/full matrix closure',
    arch=args.arch, sourceReportSha256=hashlib.sha256(native_bytes).hexdigest(), cases=cases,
    nativePngBoundsMetadataTrusted=args.arch == '386', independentFileDimensionsChecked=True)
(args.public / ('chart-readback-' + args.arch + '.json')).write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8')
print(json.dumps(dict(passed=True, arch=args.arch, nativeSaveOutputs=len(cases)*2)))
