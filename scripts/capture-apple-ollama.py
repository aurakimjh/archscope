#!/usr/bin/env python3
"""Explicit, read-only Apple/Ollama export. Never invoked by the application."""
import argparse
import ctypes
import json
import platform
import re
import subprocess
import time
import urllib.request


def command(*args):
    result = subprocess.run(args, capture_output=True, text=True, check=True, timeout=10)
    if len(result.stdout) > 65536:
        raise ValueError('command output budget')
    return result.stdout.strip()


def api(path):
    # No proxy, credentials, redirects or caller-supplied host/URL.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args, **kwargs):
            return None
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    with opener.open('http://127.0.0.1:11434/api/' + path, timeout=10) as response:
        raw = response.read(1 << 20 | 1)
        if len(raw) > 1 << 20:
            raise ValueError('API output budget')
        return json.loads(raw)


def swap():
    # Darwin sys/sysctl.h xsw_usage: bytes, not rounded sysctl -n display units.
    class Usage(ctypes.Structure):
        _fields_ = [('total', ctypes.c_uint64), ('available', ctypes.c_uint64),
                    ('used', ctypes.c_uint64), ('pagesize', ctypes.c_uint32),
                    ('encrypted', ctypes.c_uint32)]
    libc = ctypes.CDLL('/usr/lib/libSystem.B.dylib', use_errno=True)
    fn = libc.sysctlbyname
    fn.argtypes = [ctypes.c_char_p, ctypes.c_void_p, ctypes.POINTER(ctypes.c_size_t),
                   ctypes.c_void_p, ctypes.c_size_t]
    fn.restype = ctypes.c_int
    value = Usage()
    size = ctypes.c_size_t(ctypes.sizeof(value))
    if fn(b'vm.swapusage', ctypes.byref(value), ctypes.byref(size), None, 0) != 0 or size.value != ctypes.sizeof(value):
        raise OSError('vm.swapusage layout/read failed')
    return str(value.total), str(value.used)


def unavailable(reason):
    return {'value': None, 'unavailable_reason': reason}


def snapshot(model_name):
    start = str(time.time_ns())
    memory = command('/usr/sbin/sysctl', '-n', 'hw.memsize')
    vm = command('/usr/bin/vm_stat')
    match = re.search(r'page size of ([0-9]+) bytes', vm)
    if not match:
        raise ValueError('vm_stat page size missing')
    page_size = int(match[1])
    result = {'capture_start_ns': start, 'memory_capacity_bytes': memory}
    for label, key in [('Pages free', 'vm_free_bytes'), ('Pages active', 'vm_active_bytes'),
                       ('Pages inactive', 'vm_inactive_bytes'), ('Pages wired down', 'vm_wired_bytes'),
                       ('Pages occupied by compressor', 'vm_compressor_bytes')]:
        match = re.search(r'^' + label + r':\s+([0-9]+)\.$', vm, re.M)
        if not match:
            raise ValueError('vm_stat field missing: ' + label)
        result[key] = str(int(match[1]) * page_size)
    result['swap_total_bytes'], result['swap_used_bytes'] = swap()
    try:
        result['pressure_level'] = {'value': command('/usr/sbin/sysctl', '-n', 'kern.memorystatus_vm_pressure_level')}
    except subprocess.SubprocessError:
        result['pressure_level'] = unavailable('unsupported')
    try:
        thermal = command('/usr/bin/pmset', '-g', 'therm')
        match = re.search(r'^\s*Thermal_Level\s*=\s*([0-9]+)\s*$', thermal, re.M)
        result['thermal_warning_level'] = ({'value': match[1]} if match else
            unavailable('not_recorded' if 'No thermal warning level has been recorded' in thermal else 'unsupported'))
    except subprocess.SubprocessError:
        result['thermal_warning_level'] = unavailable('unsupported')
    result['gpu_utilization'] = unavailable('not_collected')
    result['power_watts'] = unavailable('permission_required')
    models = api('ps')['models']
    result['models'] = [{key: model[key] for key in ('name', 'digest', 'size', 'size_vram', 'context_length')} |
                        {'quantization_level': model['details']['quantization_level']}
                        for model in models if model['name'] == model_name]
    result['capture_end_ns'] = str(time.time_ns())
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True)
    parser.add_argument('--host-id', required=True, help='sanitized stable local host tag')
    parser.add_argument('--boot-id', required=True, help='sanitized tag unique to this OS boot')
    parser.add_argument('--model', required=True, help='exact /api/ps name to allowlist; does not load a model')
    parser.add_argument('--samples', type=int, default=3)
    parser.add_argument('--interval', type=float, default=1)
    args = parser.parse_args()
    if platform.system() != 'Darwin' or platform.machine() != 'arm64':
        parser.error('Apple Silicon macOS required')
    if not 1 <= args.samples <= 256 or not 0.2 <= args.interval <= 60:
        parser.error('samples 1..256, interval 0.2..60 seconds')
    version = api('version')['version']
    if version != '0.40.16':
        parser.error('only Ollama 0.40.16 qualified; do not override version')
    capture = {'contract': 'apple-ollama-capture.v1', 'collector_version': '1.0.0',
               'host_id': args.host_id, 'boot_id': args.boot_id, 'device_local_id': 'apple-integrated-0',
               'macos_version': command('/usr/bin/sw_vers', '-productVersion'),
               'macos_build': command('/usr/bin/sw_vers', '-buildVersion'),
               'hardware_model': command('/usr/sbin/sysctl', '-n', 'hw.model'),
               'chip': command('/usr/sbin/sysctl', '-n', 'machdep.cpu.brand_string'),
               'ollama_version': version, 'snapshots': []}
    for i in range(args.samples):
        if i:
            time.sleep(args.interval)
        capture['snapshots'].append(snapshot(args.model))
    # Exclusive publication: never truncate another file.
    with open(args.out, 'x', encoding='utf-8') as output:
        json.dump(capture, output, ensure_ascii=False, indent=2)
        output.write('\n')
    print('Saved allowlisted capture:', args.out)


if __name__ == '__main__':
    main()
