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
This file computes key metrics from Paxos client logfiles. The logfile format is
specified in src/client/client.go.
"""

import json
import numpy as np
from os import path
import statistics

def get_metrics(dirname):
    """
    Computes key metrics about an experiment from the client-side logfiles, and
    returns them as a dictionary. 'dirname' specifies the directory in which the
    client-side logfiles are stored.
    """
    with open(path.join(dirname, 'lattput.txt')) as f:
        tputs = []
        for l in f:
            l = l.split(' ')
            tputs.append(float(l[2]))

    with open(path.join(dirname, 'latency.txt')) as f:
        exec_lats = []
        commit_lats = []
        for l in f:
            l = l.split(' ')
            exec_lats.append(float(l[1]))
            commit_lats.append(float(l[2]))

    return {
        'mean_lat_commit': statistics.mean(commit_lats),
        'p50_lat_commit': np.percentile(commit_lats, 50),
        'p90_lat_commit': np.percentile(commit_lats, 90),
        'p95_lat_commit': np.percentile(commit_lats, 95),
        'p99_lat_commit': np.percentile(commit_lats, 99),
        'mean_lat_exec': statistics.mean(exec_lats),
        'p50_lat_exec': np.percentile(exec_lats, 50),
        'p90_lat_exec': np.percentile(exec_lats, 90),
        'p95_lat_exec': np.percentile(exec_lats, 95),
        'p99_lat_exec': np.percentile(exec_lats, 99),
        'avg_tput': statistics.mean(tputs),
        'total_ops': len(exec_lats),
    }

if __name__ == '__main__':
    """
    Computes client metrics from the root epaxos directory, which is where the
    files are stored on the remote client machines. Logs the metrics to stdout
    in json format.
    """
    print(json.dumps(get_metrics(path.expanduser('~/epaxos'))))
