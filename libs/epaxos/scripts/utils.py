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
This file contains miscellaneous functions that may be useful across modules.
"""

import psutil
import os
from os import path
import subprocess
import sys
import time
from tqdm import tqdm

def execute(cmd, desc):
    """
    Runs 'command' as a shell process, returning a function handler that will
    wait for the process to complete when called. 'desc' provides identifying
    information about the command.
    """
    if isinstance(cmd, list): cmd = '; '.join(cmd)

    p = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        shell=True,
        executable='/usr/local/bin/bash',
    )
    return lambda: complete_process(p, desc)

def complete_process(process, desc):
    """
    Waits for 'process', a shell process, to complete. Returns the stdout of the
    process. If the process returns an error code, prints the stderr of the
    process. 'desc' provides identifying information about the command, and is
    printed in the case of an error.
    """
    p = psutil.Process(process.pid)
    children = p.children(recursive=True)

    out, err = process.communicate()
    retcode = process.returncode
    out = out.strip()

    if retcode != 0:
        err = err.strip()
        if err: print('ERROR when completing process "{}": {}'.format(desc, err),
            file=sys.stderr)

    for cp in children:
        if psutil.pid_exists(cp.pid):
            cp.kill()

    del process
    return out

def sleep_verbose(message, delay):
    """
    Pauses program execution for 'delay' seconds. Prints '[message]: x/delay',
    where x indicates the number of seconds that have passed so far, updated
    every second.
    """
    for i in tqdm(range(delay), desc=message, total=delay,
        bar_format='{desc}: {n_fmt}/{total_fmt}'):
        time.sleep(1)

def run_verbose(message, fn, args=[]):
    """
    Calls the function provided by 'fn' with the parameters provided in 'args'.
    Prints '[message] ... done', with '[message] ...' printed at the start of
    the function call and 'done' printed at the end of the function call.
    """
    print('{} ...'.format(message), end=' ', flush=True)
    result = fn(*args)
    print('done')
    return result

def subdirectories(dirname):
    """
    Returns all directory entries that are also directories at the path provided
    by 'dirname'. Ignores the python cache.
    """
    return (d for d in os.listdir(dirname) \
        if path.isdir(path.join(dirname, d)) and d != '__pycache__')
