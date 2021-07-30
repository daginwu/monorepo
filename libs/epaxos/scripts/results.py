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
TODO
"""

import json
from os import path

from experiment import Experiment

ns_to_s = lambda x: float(x)/1e9

class Results(Experiment):
    def __init__(self, dirname):
        super().__init__(*self.dirname_to_args(dirname))
        with open(path.join(dirname, 'metrics.txt')) as f:
            self._data = json.load(f)
        self._dirname = dirname
        self._all_lats_timestamps = dict()
        self._all_lats_commit = dict()
        self._all_lats_exec = dict()

    def mean_lat_commit(self, loc):
        # TODO: fix!!
        lat = self._data[loc]['mean_lat_commit']
        if lat > 0: return lat
        else: return self.mean_lat_exec(loc)

    def p50_lat_commit(self, loc):
        return self._data[loc]['p50_lat_commit']

    def p90_lat_commit(self, loc):
        return self._data[loc]['p90_lat_commit']

    def p95_lat_commit(self, loc):
        return self._data[loc]['p95_lat_commit']

    def p99_lat_commit(self, loc):
        lat = self._data[loc]['p99_lat_commit']
        if lat > 0: return lat
        else: return self.p99_lat_exec(loc)

    def mean_lat_exec(self, loc):
        return self._data[loc]['mean_lat_exec']

    def p50_lat_exec(self, loc):
        return self._data[loc]['p50_lat_exec']

    def p90_lat_exec(self, loc):
        return self._data[loc]['p90_lat_exec']

    def p95_lat_exec(self, loc):
        return self._data[loc]['p95_lat_exec']

    def p99_lat_exec(self, loc):
        return self._data[loc]['p99_lat_exec']

    def _parse_alllats(self, loc):
        fname = path.join(self._dirname, loc, 'latency.txt')
        timestamps = []
        commit_lats = []
        exec_lats = []
        with open(fname) as f:
            for l in f:
                toks = l.split()
                timestamp = int(toks[0])
                timestamps.append(timestamp)
                exec_lat = float(toks[1])
                exec_lats.append(exec_lat)
                commit_lat = float(toks[2])
                if commit_lat > 0:
                    commit_lats.append(commit_lat)

            self._all_lats_timestamps[loc] = timestamps
            self._all_lats_commit[loc] = commit_lats
            self._all_lats_exec[loc] = exec_lats

    def all_lats_timestamps(self, loc):
        if not loc in self._all_lats_timestamps:
            self._parse_alllats(loc)

        return self._all_lats_timestamps[loc]

    def all_lats_commit(self, loc):
        if not loc in self._all_lats_commit:
            self._parse_alllats(loc)

        return self._all_lats_commit[loc]

    def all_lats_exec(self, loc):
        if not loc in self._all_lats_exec:
            self._parse_alllats(loc)

        return self._all_lats_exec[loc]

    def parse_lattput(self, loc):
        # time (ns), avg lat over the past second, tput since last line, total count, totalOrs, avg commit lat over the past second
        fname = path.join(self._dirname, loc, 'lattput.txt')
        start_time = None
        timestamps = []
        lats = []
        tputs = []
        oreqs = []
        with open(fname) as f:
            for l in f:
                toks = l.split()
                timestamp_s = ns_to_s(int(toks[0]))
                if start_time is None: start_time = timestamp_s
                timestamp_s -= start_time
                avg_lat = float(toks[1])
                avg_tput = float(toks[2])
                oreq = int(toks[4])

                timestamps.append(timestamp_s)
                lats.append(avg_lat)
                tputs.append(avg_tput)
                oreqs.append(oreq)

        return timestamps, lats, tputs, oreqs

    def avg_tput(self, loc):
        return self._data[loc]['avg_tput']

    def total_tput(self):
        return sum([self._data[loc]['avg_tput'] for loc in self._data])

    def total_ops(self, loc):
        return self._data[loc]['total_ops']

    def total_fast(self, loc):
        return self._data[loc]['total_fast']

    def total_slow(self, loc):
        return self._data[loc]['total_slow']

    def conflict_rate(self, loc):
        return self._data[loc]['conflict_rate']

    def description(self):
        if self.is_mpaxos():
            return 'MPaxos'
        elif self.is_epaxos():
            if isinstance(self._workload, self.FixedConflictWorkload):
                return '{}%'.format(self._workload._perc_conflict)
            elif isinstance(self._workload, self.ZipfianWorkload):
                return 'Zipf'
                # return 'Z.{}'.format(str(self._workload._theta)[2:]) # remove leading 0.

        # If we get here we couldn't come up with a valid name
        raise Exception()

