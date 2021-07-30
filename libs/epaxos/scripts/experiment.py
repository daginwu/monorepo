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

from os import path

EPAXOS_PROTO = 'epaxos'
MPAXOS_PROTO = 'mpaxos'
VALID_PROTOS = [EPAXOS_PROTO, MPAXOS_PROTO]

CLOCK_SYNC_NONE = 'clock-sync-none'
CLOCK_SYNC_QUORUM = 'clock-sync-quorum'
CLOCK_SYNC_QUORUM_UNION = 'clock-sync-quorum-union'
CLOCK_SYNC_CLUSTER = 'clock-sync-cluster'

TENK = 1e4
HUNDK = 1e5
MILLION = 1e6
TENMIL = 1e7
HUNDMIL = 1e8
BILLION = 1e9

LARGE_INT_TO_DESC = {
    TENK: '10K',
    HUNDK: '100K',
    MILLION: '1M',
    TENMIL: '10M',
    HUNDMIL: '100M',
    BILLION: '1B',
}

DESC_TO_LARGE_INT = { v: k for k, v in LARGE_INT_TO_DESC.items() }

class Experiment():
    class ZipfianWorkload():
        def __init__(self, frac_writes=1, unique_keys=MILLION, theta=.9):
            assert theta > 0 and theta < 1

            self._frac_writes = frac_writes
            self._unique_keys = int(unique_keys)
            self._theta = theta

        def unique_keys(self):
            return self._unique_keys

        def theta(self):
            return self._theta

        def __repr__(self):
            # No dot between unique keys and theta because theta is a float
            return 'zipf{}-{}-{}'.format(self._theta,
                LARGE_INT_TO_DESC[self._unique_keys], self._frac_writes)

    class FixedConflictWorkload():
        def __init__(self, perc_conflict, frac_writes=1):
            self._perc_conflict = perc_conflict
            self._frac_writes = frac_writes

        def perc_conflict(self):
            return self._perc_conflict

        def __repr__(self):
            return 'fixedconf{}-{}'.format(self._perc_conflict,
                self._frac_writes)

    class OutstandingReqArrivalRate():
        def __init__(self, outstanding_reqs):
            self._outstanding_reqs = outstanding_reqs

        def outstanding_reqs(self):
            return self._outstanding_reqs

        def __repr__(self):
            return 'or{}'.format(self._outstanding_reqs)

    class PoissonArrivalRate():
        def __init__(self, rate_us):
            self._rate_us = rate_us

        def rate_us(self):
            return self._rate_us

        def __repr__(self):
            return 'psn{}'.format(self._rate_us)

    def __init__(self, proto, workload, arrival_rate, inffix=True,
        batching=False, vclients=10, clock_sync=CLOCK_SYNC_NONE, thrifty=False):
        assert proto in VALID_PROTOS
        self._proto = proto
        self._workload = workload
        self._arrival_rate = arrival_rate
        self._inffix = inffix
        self._batching = batching
        self._vclients = vclients
        self._clock_sync = clock_sync
        self._thrifty = thrifty

    def batching_enabled(self):
        return self._batching

    def is_epaxos(self):
        return self._proto == EPAXOS_PROTO

    def is_mpaxos(self):
        return self._proto == MPAXOS_PROTO

    def inffix(self):
        return self._inffix

    def thrifty(self):
        return self._thrifty

    def theta(self):
        try:
            return self._workload._theta
        except:
            return None

    def vclients(self):
        return self._vclients

    def frac_writes(self):
        return self._workload._frac_writes

    def workload(self):
        return self._workload

    def arrival_rate(self):
        return self._arrival_rate

    def clock_sync_group(self):
        return {
            CLOCK_SYNC_NONE: 0,
            CLOCK_SYNC_QUORUM: 1,
            CLOCK_SYNC_CLUSTER: 2,
            CLOCK_SYNC_QUORUM_UNION: 3,
        }[self._clock_sync]

    def clock_sync_str(self):
        return self._clock_sync

    def to_dirname(self, trial):
        return '_'.join((self._proto, str(self._workload),
            str(self._arrival_rate), 'inffix' if self._inffix else 'no-inffix',
            'batch' if self._batching else 'no-batch',
            'clients{}'.format(self._vclients), self._clock_sync,
            'thrifty' if self._thrifty else 'no-thrifty', str(trial)))

    @staticmethod
    def dirname_to_args(dirname):
        """
        The format of the directory name should be:
        protocol_workload_arrival-rate_inffix_batching_vclients_trial
        """
        dirname = path.basename(path.normpath(dirname))

        proto, workload, arrival_rate, inffix, batching, vclients, clock_sync, \
            thrifty, trial = dirname.split('_')

        if workload.startswith('fixedconf'):
            workload = workload.strip('fixedconf')
            perc_conflict, perc_writes = workload.split('-')
            perc_conflict = int(perc_conflict)
            perc_writes = int(perc_writes)
            workload = Experiment.FixedConflictWorkload(perc_conflict, perc_writes)
        elif workload.startswith('zipf'):
            workload = workload.strip('zipf')
            theta, unique_keys, frac_writes = workload.split('-')
            frac_writes = float(frac_writes)
            theta = float(theta)
            unique_keys = DESC_TO_LARGE_INT[unique_keys]
            workload = Experiment.ZipfianWorkload(frac_writes, unique_keys, theta)
        else:
            raise Exception()

        if arrival_rate.startswith('psn'):
            arrival_rate = Experiment.PoissonArrivalRate(int(arrival_rate.strip('psn')))
        elif arrival_rate.startswith('or'):
            arrival_rate = \
                Experiment.OutstandingReqArrivalRate(int(arrival_rate.strip('or')))
        else:
            raise Exception()

        inffix = inffix == 'inffix'
        batching = batching == 'batch'
        vclients = int(vclients.strip('clients'))
        thrifty = thrifty == 'thrifty'

        return proto, workload, arrival_rate, inffix, batching, vclients, clock_sync, thrifty

    @classmethod
    def from_dirname(cls, dirname):
        return cls(*cls.dirname_to_args(dirname))
