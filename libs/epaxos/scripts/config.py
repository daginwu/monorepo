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
This file contains user-specific constants and functions. These should be
updated when the repo is cloned before running the scripts.
"""

"""
This constant should be set to be the ID of the Google Cloud Compute Engine
project that you want to use to run the experiments.
"""
GCLOUD_PROJECT_ID = 'TODO'

"""
This constant should be set to the expanded path of the directory of the cloned
epaxos repo. (This will likely be one directory up from the directory containing
this file.)
"""
EPAXOS_DIR = 'TODO'

def download_clock_sync_software(instance):
    """
    Given an GCloudInstance object 'instance', downloads (or copies from the
    local machine) the necessary software to that instance so that it can
    synchronize its clocks with other machines. Expected to be asynchronous and
    return a function handler that will wait for the process to complete when
    called.
    """
    # TODO: return utils.execute(command, desc)
    pass

def install_clock_sync_software(instance):
    """
    Given a GCloudInstance object 'instance' that already has the clock
    synchronization software downloaded, installs that software. Expected to be
    asynchronous and return a function handler that will wait for the process to
    complete when called.
    """
    # TODO: return utils.execute(command, desc)
    pass

def reset_clock_sync(instance):
    """
    Given a GCloudInstance object 'instance' (server or master), kills any clock
    synchronization software that may be running and removes and reminants of
    the software, such that it will run cleanly on the next attempt. Expected to
    be asynchronous and return a function handler that will wait for the process
    to complete when called.
    """
    # TODO: return utils.execute(command, desc)
    pass

def synchronize_clocks_master(master):
    """
    Given a GCloudMaster object 'master', runs the clock synchronization master
    software. Expected to be asynchronous and non-blocking.
    """
    # TODO: return utils.execute(command, desc)
    pass

def synchronize_clocks_server(server, master_ip):
    """
    Given a GCloudServer object 'server' and the string IP address 'master_ip'
    of the GCloudMaster for the topology , which will be running its clock
    synchronization software, runs the clock synchronization software for a
    server. Expected to be asynchronous and non-blocking.
    """
    # TODO: return utils.execute(command, desc)
    pass
