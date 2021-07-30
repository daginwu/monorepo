#!/usr/local/bin/python3

# Copyright (c) 2020 Stanford University
#
# Permission to use, copy, modify, and distribute this software for any
# purpose with or without fee is hereby granted, provided that the above
# copyright notice and this permission notice appear in all copies.
#
# THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR(S) DISCLAIM ALL WARRANTIES
# WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
# MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL AUTHORS BE LIABLE FOR
# ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
# WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
# ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
# OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

"""
Manages a Google Cloud topology associated with a Paxos cluster and runs
experiments on it.
"""

import argparse
import json
import multiprocessing
import operator
import os
from os import path
import socket
import struct

import config
import experiment
from experiment import Experiment
import utils

LOCATION_TO_INDEX = {
    'ca': 0,
    'va': 1,
    'eu': 2,
    'or': 3,
    'jp': 4,
}

class GCloudInstance():
    """
    """

    def __init__(self, loc):
        self._loc = loc
        self._internal_ip = None
        self._external_ip = None

    def run(self):
        # TODO: run should take in an experiment or server/client
        raise NotImplementedError()

    def ip(self, internal=True):
        if internal and self._internal_ip: return self._internal_ip
        if not internal and self._external_ip: return self._external_ip

        command = 'gcloud compute instances list --filter="name={}" ' \
            '--format "get(networkInterfaces[0].{})"'.format(self.id(),
                'networkIP' if internal else 'accessConfigs[0].natIP')
        desc = '{} {} IP address'.format('internal' if internal else 'external',
            self.id())

        ip = utils.execute(command, desc)()

        if internal: self._internal_ip = ip
        else: self._external_ip = ip

        return ip

    def _gssh(self, cmd, desc):
        # To see the commands that are run on each machine, uncomment the
        # statements below.
        # print(cmd)
        # return lambda: None
        return utils.execute(self._gssh_cmd(cmd), '{}: {}'.format(self.id(),
            desc))

    def _gssh_cmd(self, cmd):
        if isinstance(cmd, list): cmd = '; '.join(cmd)

        return 'gcloud compute ssh {} --zone {} --command=\'{}\''.format(
            self.id(),
            self.zone(),
            cmd
        )

    def zone(self):
        return {
            'ca': 'us-west2-b',
            'va': 'us-east4-a',
            'eu': 'europe-west2-a',
            'or': 'us-west1-a',
            'jp': 'asia-northeast1-a',
        }[self._loc]

    def create(self):
        return utils.execute('gcloud compute instances create {} '
            '--zone={} '
            '--machine-type=n1-standard-8 '
            '--image-family=ubuntu-1804-lts '
            '--image-project=ubuntu-os-cloud '.format(self.id(),
                self.zone()), '{}: creating machine'.format(self.id()))

    def start(self):
        return utils.execute('gcloud compute instances start '
            '{} --zone {}'.format(self.id(), self.zone()),
            '{}: starting machine'.format(self.id()))

    def stop(self):
        return utils.execute('gcloud compute instances stop '
            '{} --zone {}'.format(self.id(), self.zone()),
            '{}: stopping machine'.format(self.id()))

    def install_go(self):
        return self._gssh([
            'sudo apt-get purge golang -y',
            'sudo add-apt-repository ppa:longsleep/golang-backports -y',
            'sudo apt-get update -y',
            'sudo apt-get install golang-go -y',
        ], 'Installing go')

    def download_packages(self):
        return self._gssh([
            'export GOPATH=~/epaxos',
            'go get golang.org/x/sync/semaphore',
            'go get -u google.golang.org/grpc',
            'go get -u github.com/golang/protobuf/protoc-gen-go',
            'go get -u github.com/VividCortex/ewma',
            'export PATH=$PATH:$GOPATH/bin',
            # For client metrics script
            'sudo apt-get install python3-pip -y && pip3 install numpy'
        ], 'Downloading packages')


    def rsync(self, install=False):
        # This is NOT SECURE, but makes it possible to connect
        sshopts = 'ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'
        remote = '{}.{}.{}'.format(self.id(), self.zone(),
            config.GCLOUD_PROJECT_ID)
        rsync_command = 'rsync --delete --exclude-from "{f}/.gitignore" ' \
            '-re "{sshopts}" {f} {remote}:~'.format(sshopts=sshopts,
            f=config.EPAXOS_DIR, remote=remote)
        gocmd = ['export GOPATH=~/epaxos']
        if install:
            gocmd.extend([
                'go get golang.org/x/sync/semaphore',
                'go get -u google.golang.org/grpc',
                'go get -u github.com/golang/protobuf/protoc-gen-go',
                'go get -u github.com/VividCortex/ewma',
                'export PATH=$PATH:$GOPATH/bin'
            ])
        gocmd.extend([
            'go clean',
            'go install master',
            'go install server',
            'go install client'
        ])

        install_command = self._gssh_cmd(gocmd)

        return utils.execute([rsync_command, install_command], 'Rsync')

    def kill(self):
        return NotImplementedError()

    def metrics_filenames(self):
        return NotImplementedError()

    # Returns a handler for when we want to complete the processes
    # If start is true, the file sizes are stored as the start points, otherwise
    # as the end points.
    def get_file_sizes(self, start):
        processes = [
            (self._gssh('stat -c %s epaxos/{}'.format(f),
                '{} size'.format(f)), f) for f in self.metrics_filenames()
        ]

        def handler():
            sizes = {
                f : p() for p, f in processes
            }
            if start:
                self._file_starts = sizes
            else:
                self._file_ends = sizes

        return handler

    def trim_files(self):
        assert (self._file_starts is not None and self._file_ends is not None)

        trim_file = lambda f: self._gssh('cp epaxos/{fname} '
            'epaxos/{fname}_full && truncate -s {end} epaxos/{fname} && '
            'echo "$(tail -c +{start} epaxos/{fname})" > epaxos/{fname} && '
            'sed -i "1d;$d" epaxos/{fname}'.format(fname=f,
                start=self._file_starts[f], end=self._file_ends[f]),
                'trimming {}'.format(f))
        processes = [trim_file(f) for f in self.metrics_filenames()]

        def handler():
            for p in processes: p()
        return handler

    def copy_output_file(self, dst_dirname):
        raise NotImplementedError()

    def copy_files(self, dst_dirname):
        copy_file = lambda f: \
            utils.execute('gcloud compute scp {}:epaxos/{} {} --zone {}'.format(
                self.id(),
                f,
                path.join(dst_dirname, f),
                self.zone()
            ), '{}: copying metrics files'.format(self.id()))

        processes = list(map(copy_file, self.metrics_filenames()))

        def handler():
            for p in processes: p()
        return handler

class GCloudServer(GCloudInstance):
    def id(self):
        return 'server-{}'.format(self._loc)

    def flags(self, master_ip, expt):
        port = 7070 + LOCATION_TO_INDEX[self._loc]
        flags = [
            '-port {}'.format(port),
            '-maddr {}'.format(master_ip),
            '-addr {}'.format(self.ip(internal=True)),
            '-clocksync {} -clockepsilon 0'.format(expt.clock_sync_group()),
        ]
        if expt.is_epaxos(): flags.append('-e')
        if expt.batching_enabled(): flags.append('-batch')
        if expt.inffix(): flags.append('-inffix')
        if expt.thrifty(): flags.append('-beacon -thrifty')

        return ' '.join(flags)

    def run(self, master_ip, expt):
        flags = self.flags(master_ip, expt)
        return self._gssh('cd epaxos && bin/server {} > output.txt 2>&1'.format(flags), 'run')

    def kill(self):
        return self._gssh('kill $(pidof bin/server)', 'force kill')

    def get_conflict_rate(self, start):
        def target(q):
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                port = 7070 + LOCATION_TO_INDEX[self._loc]
                s.connect((self.ip(internal=False), port))
                s.sendall(struct.pack('!BB', 4, 0))
                data = s.recv(1+8*2)
                kFast, kSlow = struct.unpack('qq', data)
                q.put((kFast, kSlow))

        q = multiprocessing.Queue()
        p = multiprocessing.Process(target=target, args=(q,))
        p.start()
        def handler():
            p.join(10)
            if p.is_alive():
                p.kill()
                raise Exception()
            if p.exitcode != 0:
                raise Exception()
            resp = q.get()
            if start:
                self._start_conflict = resp
            else:
                self._end_conflict = resp
        return handler

    def compute_conflict(self):
        assert self._start_conflict and self._end_conflict

        total_fast = self._end_conflict[0] - self._start_conflict[0]
        total_slow = self._end_conflict[1] - self._start_conflict[1]
        total = total_fast + total_slow
        conflict = total_slow/total if total > 0 else 0
        return total_fast, total_slow, conflict

    def copy_output_file(self, dst_dirname):
        return utils.execute('gcloud compute scp {}:epaxos/output.txt {} --zone {}'.format(
                self.id(),
                path.join(dst_dirname, 'server_output.txt'),
                self.zone()
            ), '{}: copying output file'.format(self.id()))

# Master is co-located with server
class GCloudMaster(GCloudServer):
    def run_master(self, server_ips):
        return self._gssh('cd epaxos && bin/master -N {} -ips {} > moutput.txt 2>&1'.format(
            len(server_ips), ','.join(server_ips)), 'run')

    def kill(self):
        return self._gssh('kill $(pidof bin/master) $(pidof bin/server)', 'force kill')

    def copy_output_file(self, dst_dirname):
        return utils.execute('gcloud compute scp {}:epaxos/moutput.txt {} --zone {}'.format(
                self.id(),
                path.join(dst_dirname, 'master_output.txt'),
                self.zone()
            ), '{}: copying output file'.format(self.id()))

class GCloudClient(GCloudInstance):
    def id(self):
        return 'client-{}'.format(self._loc)

    def metrics_filenames(self):
        return ['latency.txt', 'lattput.txt']

    def flags(self, master_ip, expt):
        flags = [
            '-maddr {}'.format(master_ip),
            '-T {}'.format(expt.vclients()),
            '-writes {}'.format(expt.frac_writes()),
        ]

        # force leader
        if expt.is_epaxos():
            flags.append(' -l {}'.format(LOCATION_TO_INDEX[self._loc]))

        workload = expt.workload()
        if isinstance(workload, Experiment.FixedConflictWorkload):
            # don't overlap start regions
            flags.append('-sr {}'.format(LOCATION_TO_INDEX[self._loc] * 500000))
            flags.append('-c {}'.format(workload.perc_conflict()))
        elif isinstance(workload, Experiment.ZipfianWorkload):
            flags.append('-c -1')
            flags.append('-z {}'.format(workload.unique_keys()))
            flags.append('-theta {}'.format(workload.theta()))

        arrival_rate = expt.arrival_rate()
        if isinstance(arrival_rate, Experiment.OutstandingReqArrivalRate):
            flags.append('-or {}'.format(arrival_rate.outstanding_reqs()))
        elif isinstance(arrival_rate, Experiment.PoissonArrivalRate):
            # flags.append('-or {}'.format(int(15000/expt.vclients()))) # Cap poisson at 1500
            flags.append('-or {}'.format(int(10))) # 16
            flags.append('-poisson {}'.format(arrival_rate.rate_us()))

        return ' '.join(flags)

    def run(self, master_ip, expt):
        flags = self.flags(master_ip, expt)
        return self._gssh('cd epaxos && bin/client {} > output.txt 2>&1'.format(flags), 'run')

    def kill(self):
        return self._gssh('kill $(pidof bin/client)', 'force kill')

    def get_metrics(self):
        return self._gssh('python3 epaxos/scripts/client_metrics.py',
            'getting metrics')

    def copy_output_file(self, dst_dirname):
        return utils.execute('gcloud compute scp {}:epaxos/output.txt {} --zone {}'.format(
                self.id(),
                path.join(dst_dirname, 'client_output.txt'),
                self.zone()
            ), '{}: copying output file'.format(self.id()))

class GCloudTopology():
    def __init__(self, locs=['ca', 'va', 'eu', 'or', 'jp']):
        assert len(locs) > 0

        self._locs = locs
        self._master = GCloudMaster(locs[0])
        self._servers = [self._master] + [GCloudServer(l) for l in locs[1:]]
        self._clients = [GCloudClient(l) for i, l in enumerate(locs)]
        self._instances = self._servers + self._clients

    def _run_all(self, method, args=[], instances=None):
        if instances is None: instances = self._instances

        get_method = operator.methodcaller(method, *args)
        return list(map(get_method, instances)) # converting to list gets it to actually run

    def _complete_all(self, method, args=[], instances=None):
        for handler in self._run_all(method, args, instances): handler()

    def create(self):
        self._complete_all('create')

    def install_go(self):
        self._complete_all('install_go')

    def download_packages(self):
        self._complete_all('download_packages')

    def _expose_ports(self, rulename, ports, instances, description):
        return utils.execute('gcloud compute firewall-rules create {rulename} --allow '
            'tcp:{ports} --source-tags={instances} --source-ranges=0.0.0.0/0 '
            '--description="{description}"'.format(rulename=rulename,
                ports=ports, instances=','.join([i.id() for i in instances]),
                description=description),
            'Exposing ports with description "{}"'.format(description))

    def expose_ports(self):
        """
        Exposes ports on each server so that we can be a mock client and get the
        conflict rate metrics.
        """
        self._expose_ports('mock-client', '7000-8000', self._servers,
            'Ports through which clients contact servers')()
        self._expose_ports('clock-sync-ui', '9001', [self._master],
            'Port for the clock synchronization software UI')()

    def rsync(self, install=False):
        utils.execute('gcloud compute config-ssh', 'gssh init')()
        self._complete_all('rsync', args=[install])

    def download_clock_sync(self):
        for handler in list(map(config.download_clock_sync_software,
            self._servers)): handler()

    def install_clock_sync(self):
        for handler in list(map(config.install_clock_sync_software,
            self._servers)): handler()

    def synchronize_clocks(self):
        for handler in list(map(config.reset_clock_sync,
            self._servers)): handler()

        print('Synchronizing clocks ...')

        config.synchronize_clocks_master(self._master)
        utils.sleep_verbose('Wait for master to sync', 10)

        for s in self._servers:
            config.synchronize_clocks_server(s, self._master.ip())

        print('Server clocks should be synced. UI available at {}:9001'.format(
            self._master.ip(internal=False)))

    def start(self):
        self._complete_all('start')

    def stop(self):
        self._complete_all('stop')

    def kill(self):
        self._complete_all('kill')

    def run(self, expt, dirname, stabilize_delay=5, capture_delay=5, full_metrics=False): # TODO: change full metrics to FALSE
        with open(path.join(dirname, 'flags.txt'), 'w') as f:
            print('Server flags: {}\nClient flags: {}'.format(
                self._servers[0].flags(self._master.ip(internal=True), expt),
                self._clients[0].flags(self._master.ip(internal=True), expt)),
                    file=f)

        try:
            utils.run_verbose('Starting master', self._master.run_master,
                [[_.ip() for _ in self._servers]])
            utils.run_verbose('Starting servers', self._run_all, ['run',
                [self._master.ip(), expt], self._servers])
            utils.sleep_verbose('Letting servers connect', 5)
            utils.run_verbose('Starting clients', self._run_all, ['run',
                [self._master.ip(), expt], self._clients])

            utils.sleep_verbose('Stabilizing', stabilize_delay)

            utils.run_verbose('Getting conflict rate', self._complete_all,
                ['get_conflict_rate', [True], self._servers])
            utils.run_verbose('Getting file sizes', self._complete_all,
                ['get_file_sizes', [True], self._clients])

            utils.sleep_verbose('Capturing', capture_delay)

            utils.run_verbose('Getting conflict rate', self._complete_all,
                ['get_conflict_rate', [False], self._servers])
            utils.run_verbose('Getting file sizes', self._complete_all,
                ['get_file_sizes', [False], self._clients])

        finally:
            # Kill all machines to cleanup when interrupted
            utils.run_verbose('Killing Paxos processes', self.kill)

        utils.run_verbose('Trimming files', self._complete_all, ['trim_files',
                [], self._clients])

        def get_metrics():
            handlers = [c.get_metrics() for c in self._clients]
            metrics = dict()
            for i, h in enumerate(handlers):
                metrics[self._locs[i]] = json.loads(h())
                total_fast, total_slow, conflict = \
                    self._servers[i].compute_conflict()
                metrics[self._locs[i]]['total_fast'] = total_fast
                metrics[self._locs[i]]['total_slow'] = total_slow
                metrics[self._locs[i]]['conflict_rate'] = conflict
            with open(path.join(dirname, 'metrics.txt'), 'w') as f:
                print(json.dumps(metrics, indent=4), file=f)
        utils.run_verbose('Getting metrics', get_metrics)

        if full_metrics:
            def download_files():
                handlers = []
                for i, c in enumerate(self._clients):
                    metrics_dir = path.join(dirname, self._locs[i])
                    os.mkdir(metrics_dir)
                    h = c.copy_files(metrics_dir)
                    handlers.append(h)
                    handlers.append(c.copy_output_file(metrics_dir))
                    handlers.append(self._servers[i].copy_output_file(metrics_dir))
                handlers.append(self._master.copy_output_file(dirname))
                for h in handlers: h()
            utils.run_verbose('Downloading metrics files', download_files)


if __name__ == '__main__':
    """
    Performs the action on the Google Cloud topology that is associated with the
    provided command line argument.
    """
    parser = argparse.ArgumentParser()
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--start', help='Power on Google Cloud instances',
        action='store_true')
    group.add_argument('--clock_sync', help='Synchronize clocks of servers',
        action='store_true')
    group.add_argument('--stop', help='Power off Google Cloud instances',
        action='store_true')
    group.add_argument('--cleanup', help='Manually kill all Paxos processes '
        'running on Google Cloud instances', action='store_true')
    group.add_argument('--create', help='Instantiates Google Cloud instances '
        'associated with the test topology and provisions them so that they '
        'are ready to run experiments', action='store_true')

    args = parser.parse_args()

    topo = GCloudTopology()

    if args.create:
        # This is broken up into so many different commands so that if something
        # times out or fails it is easy to pinpoint which command it was and
        # restart just that portion.
        utils.run_verbose('Creating instances', topo.create)
        utils.run_verbose('Exposing ports', topo.expose_ports)
        utils.run_verbose('Installing go', topo.install_go)
        utils.run_verbose('Rsyncing instances', topo.rsync, [True])
        utils.run_verbose('Downloading packages', topo.download_packages)
        utils.run_verbose('Downloading clock synchronization software',
            topo.download_clock_sync)
        utils.run_verbose('Installing clock synchronization software',
            topo.install_clock_sync)

    elif args.start:
        utils.run_verbose('Starting machines', topo.start)

    elif args.clock_sync:
        topo.synchronize_clocks()

    elif args.stop:
        utils.run_verbose('Stopping machines', topo.stop)

    elif args.cleanup:
        utils.run_verbose('Cleanup machines', topo.kill)

