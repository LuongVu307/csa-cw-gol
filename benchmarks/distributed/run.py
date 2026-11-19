#!/usr/bin/env python3
"""Run isolated localhost measurements with two and four Go worker processes."""
import argparse
import hashlib
import os
from pathlib import Path
import platform
import shutil
import socket
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[2]
BROKER = ROOT.parent / 'csa-cw-gol-distributor-server'
WORKER = ROOT.parent / 'csa-gol-cw-worker-server'


def execute(command, **kwargs):
    return subprocess.run(command, check=True, **kwargs)


def free_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def start(binary, port, directory, logfile, environment):
    stream = logfile.open('w')
    try:
        process = subprocess.Popen([str(binary), str(port)], cwd=directory,
                                   stdout=stream, stderr=subprocess.STDOUT, env=environment)
    finally:
        stream.close()
    return process


def wait_ready(process, port):
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f'Server exited on port {port}; inspect server logs')
        try:
            with socket.create_connection(('127.0.0.1', port), timeout=.2):
                return
        except OSError:
            time.sleep(.05)
    raise RuntimeError(f'Server did not become ready on port {port}')


def stop(processes):
    # These servers handle SIGTERM as a warning first, so force-stop only the
    # processes started by this script, after giving them a chance to exit.
    for process in processes:
        if process.poll() is None:
            process.terminate()
    for process in processes:
        try:
            process.wait(timeout=.5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, help='New results directory')
    args = parser.parse_args()
    # Fail before building or starting anything if the environment blocks TCP.
    free_port()
    for directory in (BROKER, WORKER):
        if not (directory / 'go.mod').is_file():
            parser.error(f'Missing sibling server repository: {directory}')
    output = (args.output or ROOT / 'benchmarks/distributed/runs' / time.strftime('%Y%m%d-%H%M%S')).resolve()
    output.mkdir(parents=True, exist_ok=False)
    environment = dict(os.environ, GOMAXPROCS='1')
    setup = f'localhost TCP; host={platform.node()}; GOMAXPROCS=1 per controller, broker and worker process'
    with (output / 'environment.txt').open('w') as record:
        record.write(f'UTC: {time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}\n')
        record.write(f'{setup}\n{platform.platform()}\nLogical CPUs: {os.cpu_count()}\n')
        record.write('Sequential configurations: 2 then 4 workers; 3 samples each; 512x512; 100 turns.\n')
        record.write('All processes share one machine. This is not AWS or multi-host scaling.\n')
        if Path('/proc/cpuinfo').exists():
            model = next((line for line in Path('/proc/cpuinfo').read_text().splitlines() if line.startswith('model name')), '')
            record.write(model+'\n')
        execute(['go', 'version'], stdout=record)
        for directory in (ROOT, BROKER, WORKER):
            record.write(f'\nRepository: {directory}\n'); record.flush()
            execute(['git', 'rev-parse', 'HEAD'], cwd=directory, stdout=record)
            execute(['git', 'status', '--short'], cwd=directory, stdout=record)
            for source in sorted(directory.rglob('*.go')):
                if '.git' not in source.parts:
                    record.write(f'{hashlib.sha256(source.read_bytes()).hexdigest()}  {source}\n')
        for fixture in (ROOT / 'images/512x512.pgm', ROOT / 'check/images/512x512x100.pgm'):
            record.write(f'{hashlib.sha256(fixture.read_bytes()).hexdigest()}  {fixture}\n')
    with tempfile.TemporaryDirectory(prefix='cw1-distributed-') as temporary:
        temporary = Path(temporary)
        binaries = {}
        for name, directory, package in [('worker', WORKER, '.'), ('broker', BROKER, '.'), ('measure', ROOT, './benchmarks/distributed')]:
            binaries[name] = temporary / name
            print(f'Building {name}...', flush=True)
            execute(['go', 'build', '-o', str(binaries[name]), package], cwd=directory, env=environment)
        for count in (2, 4):
            processes = []
            directory = temporary / f'workers-{count}'
            directory.mkdir()
            try:
                addresses = []
                for index in range(count):
                    port = free_port()
                    process = start(binaries['worker'], port, directory, output / f'{count}-worker-{index}.log', environment)
                    processes.append(process)
                    wait_ready(process, port)
                    addresses.append(f'127.0.0.1:{port}')
                (directory / 'workerURLs.txt').write_text('\n'.join(addresses)+'\n')
                broker_port = free_port()
                broker = start(binaries['broker'], broker_port, directory, output / f'{count}-broker.log', environment)
                processes.append(broker)
                wait_ready(broker, broker_port)
                print(f'Measuring {count} workers, three samples...', flush=True)
                with (output / f'workers-{count}.log').open('w') as log:
                    execute([str(binaries['measure']), '-broker', f'127.0.0.1:{broker_port}',
                             '-workers', str(count), '-count', '3', '-setup', setup,
                             '-output', str(output / f'workers-{count}.csv')],
                            cwd=ROOT, env=environment, stdout=log, stderr=subprocess.STDOUT, timeout=180)
                print((output / f'workers-{count}.log').read_text(), end='', flush=True)
            finally:
                stop(processes)
    execute([sys.executable, str(ROOT / 'benchmarks/distributed/analyze.py'),
             str(output / 'workers-2.csv'), str(output / 'workers-4.csv'),
             '--output', str(output / 'summary')])
    if shutil.which('pdflatex'):
        with (output / 'summary/build.log').open('w') as log:
            execute(['pdflatex', '-interaction=nonstopmode', '-halt-on-error', 'runtime.tex'],
                    cwd=output / 'summary', stdout=log, stderr=subprocess.STDOUT)
    print(f'\nResults: {output}\nSummary: {output / "summary/summary.csv"}', flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(f'Measurement stopped: {error}\nNo failed sample should be reported as a successful timing.', file=sys.stderr)
        sys.exit(1)
