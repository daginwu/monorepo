EPaxos
======

This repo contains code used to evaluate the Egalitarian Paxos consensus
protocol for the NSDI '21 paper [EPaxos Revisited](https://www.usenix.org/conference/nsdi21/presentation/tollman).
This is a fork of the code used for the original EPaxos evaluation for
[the EPaxos SOSP paper](http://dl.acm.org/ft_gateway.cfm?id=2517350&ftid=1403953&dwn=1).

The `scripts` directory contains code that produces the results of our
re-evaluation. The experiments are configured for Google Cloud, so you will need
a valid Google Cloud project id in order to run them.
You will need to update the constants in `config.py` before running the scripts.
(`git update-index --skip-worktree scripts/config.py` ignores changes to that
file.)

`python scripts/gcloud_topology.py --create` creates instances of the correct
types in the correct locations.

`python main.py` runs experiments from our re-evaluation and generates
graphs of the results.
