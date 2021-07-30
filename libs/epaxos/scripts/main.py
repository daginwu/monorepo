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
TODO(sktollman): add file comment
"""

import argparse
import os, sys
from os import path
import shutil

from experiment import Experiment, MPAXOS_PROTO, EPAXOS_PROTO, \
    CLOCK_SYNC_NONE, CLOCK_SYNC_QUORUM, CLOCK_SYNC_QUORUM_UNION, \
    CLOCK_SYNC_CLUSTER
from gcloud_topology import GCloudTopology
import graphs
import utils

topo = GCloudTopology()

def run(expts, root_dirname, full_results=False, trials=1):
    if not path.exists(root_dirname):
        os.mkdir(root_dirname)

    for i in range(trials):
        for expt in expts:
            expt_dirname = path.join(root_dirname, expt.to_dirname(i))

            # If the directory already exists, skip it because the experiment
            # has already been run
            if path.exists(expt_dirname):
                continue

            if expt != expts[0]:
                utils.sleep_verbose('Letting machines chill', 5)

            for attempt in range(3): # 3 attempts
                try:
                    os.mkdir(expt_dirname)

                    print('## {} ##'.format(expt.to_dirname(i)))

                    topo.run(expt, expt_dirname, stabilize_delay=20,
                        capture_delay=60, full_metrics=full_results)
                    break # Ran successfully, don't need to attempt again
                except (KeyboardInterrupt, SystemExit):
                    raise
                except:
                    shutil.rmtree(expt_dirname)
                    print('** RETRYING **', file=sys.stderr)

            # If we didn't break out of the loop, we couldn't successfully
            # run the expt
            else:
                print('** FAILED **', file=sys.stderr)

            print() # Newline between experiments

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--rsync', help='Whether to copy the latest version of '
        'the files with all machines', default=False, action='store_true')
    parser.add_argument('--off', help='Whether to power off the machines at the'
        ' end of running the experiments', default=False, action='store_true')
    args = parser.parse_args()
    if args.rsync:
        utils.run_verbose('Rsyncing instances', topo.rsync)
        print()

    try:
        print('##################')
        print('## Reproduction ##')
        print('##################')
        repro_arrival_rate = Experiment.OutstandingReqArrivalRate(1)
        reproduction_expts = [
            Experiment(MPAXOS_PROTO, Experiment.ZipfianWorkload(),
                repro_arrival_rate),
            Experiment(EPAXOS_PROTO, Experiment.ZipfianWorkload(),
                repro_arrival_rate),
            Experiment(EPAXOS_PROTO, Experiment.FixedConflictWorkload(0),
                repro_arrival_rate),
            Experiment(EPAXOS_PROTO, Experiment.FixedConflictWorkload(2),
                repro_arrival_rate),
            Experiment(EPAXOS_PROTO, Experiment.FixedConflictWorkload(100),
                repro_arrival_rate),
        ]
        run(reproduction_expts, 'results/reproduction_1or', trials=1)
        graphs.reproduction_bar('results/reproduction_1or')

        print('##############')
        print('## Batching ##')
        print('##############')
        # .9, 50% writes, batching, no batching, mpaxos, psn 4500
        default_arrival_rate = Experiment.PoissonArrivalRate(4500)
        batching_workload = Experiment.ZipfianWorkload(theta=.9, frac_writes=.5)
        batching_expts = [
            Experiment(MPAXOS_PROTO, batching_workload, default_arrival_rate),
            Experiment(EPAXOS_PROTO, batching_workload, default_arrival_rate),
            Experiment(EPAXOS_PROTO, batching_workload, default_arrival_rate,
                batching=True),
        ]
        run(batching_expts, 'results/batching', trials=5)
        graphs.batching_bar('results/batching')

        print('#########')
        print('## OSC ##')
        print('#########')
        default_arrival_rate = Experiment.PoissonArrivalRate(4500)
        osc_workload = Experiment.ZipfianWorkload(theta=.99, frac_writes=1)
        osc_expts = [
            Experiment(MPAXOS_PROTO, osc_workload, default_arrival_rate),
        ]
        for w in [
            osc_workload,
            Experiment.ZipfianWorkload(theta=.8, frac_writes=.5),
            Experiment.ZipfianWorkload(theta=.99, frac_writes=1)
        ]:
            osc_expts.extend([
                Experiment(EPAXOS_PROTO, w, default_arrival_rate,
                    clock_sync=CLOCK_SYNC_NONE),
                Experiment(EPAXOS_PROTO, w, default_arrival_rate,
                    clock_sync=CLOCK_SYNC_QUORUM),
                Experiment(EPAXOS_PROTO, w, default_arrival_rate,
                    clock_sync=CLOCK_SYNC_QUORUM_UNION),
                Experiment(EPAXOS_PROTO, w, default_arrival_rate,
                    clock_sync=CLOCK_SYNC_CLUSTER),
            ])
        run(osc_expts, 'results/osc', trials=5)
        graphs.osc_bar('results/osc')

        print('##############')
        print('## Thrifty ##')
        print('##############')
        default_arrival_rate = Experiment.PoissonArrivalRate(4500)
        thrifty_expts = [
            Experiment(EPAXOS_PROTO, Experiment.ZipfianWorkload(theta=.9,
                frac_writes=.5), default_arrival_rate),
            Experiment(EPAXOS_PROTO, Experiment.ZipfianWorkload(theta=.9,
                frac_writes=.5), default_arrival_rate, thrifty=True),

            Experiment(EPAXOS_PROTO, Experiment.ZipfianWorkload(theta=.99,
                frac_writes=1), default_arrival_rate),
            Experiment(EPAXOS_PROTO, Experiment.ZipfianWorkload(theta=.99,
                frac_writes=1), default_arrival_rate, thrifty=True),

            Experiment(EPAXOS_PROTO, Experiment.ZipfianWorkload(theta=.7,
                frac_writes=.3), default_arrival_rate),
            Experiment(EPAXOS_PROTO, Experiment.ZipfianWorkload(theta=.7,
                frac_writes=.3), default_arrival_rate, thrifty=True),
        ]
        run(thrifty_expts, 'results/thrifty', trials=5)
        graphs.thrifty_bar('results/thrifty')

    finally:
        os.system('afplay /System/Library/Sounds/Sosumi.aiff') # Beep when done

    if args.off:
        utils.run_verbose('Powering off instances', topo.stop)
        print()
