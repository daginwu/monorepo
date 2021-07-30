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
This file contains code that plots the graphs in the "EPaxos Revisited" paper.
"""

import matplotlib
from matplotlib import lines, patches, ticker
import matplotlib.pyplot as plt
import numpy as np
from os import path
import statistics
from tabulate import tabulate

from experiment import Experiment, CLOCK_SYNC_NONE, CLOCK_SYNC_QUORUM, \
    CLOCK_SYNC_QUORUM_UNION, CLOCK_SYNC_CLUSTER
from results import Results
import utils

# The list of client locations, in the same order as the original EPaxos paper
ORDERED_LOCS = ['va', 'ca', 'or', 'jp', 'eu']
# A color to use when differentiating from the default EPaxos Zipfian experiment
ALTERNATE_ZIPF_COLOR = '#DA70D6' # Light purple
# Fill patterns in a consistent order
HATCHES = [None, '//', None]# TODO: revert!! '.']

# Set a consistent font for all graphs
matplotlib.rc('font',**{'family':'sans-serif','sans-serif':['Helvetica']})
plt.rcParams['pdf.fonttype'] = 42
plt.rcParams['font.family'] = 'Helvetica'

def get_results(root_dirname):
    """
    Returns a list of Results objects for each subdirectory from 'root_dirname'.
    This is typically a set of experiments that will be plotted together on a
    graph.
    """
    return list(map(Results, map(lambda dirname: path.join(root_dirname,
        dirname), utils.subdirectories(root_dirname))))

def is_fixed_epaxos_result(r, perc_conflict):
    """
    Returns True if 'r', a Results object, contains results for an experiment in
    which the protocol was EPaxos and the workload fixed at the percentage
    provided by 'perc_conflict'.
    """
    return r.is_epaxos() and \
        isinstance(r.workload(), Experiment.FixedConflictWorkload) and \
        r.workload().perc_conflict() == perc_conflict

def get_fixed_epaxos_result(results, perc_conflict):
    """
    Returns a Results object from the 'results' list in which the protocol was
    EPaxos and the workload fixed at the percentage provided by 'perc_conflict'.
    """
    return list(filter(lambda r: is_fixed_epaxos_result(r, perc_conflict),
        results))

def is_epaxos_zipf_result(r):
    """
    Returns True if the the Results object 'r' was an experiment for the EPaxos
    protocol and the workload was Zipfian.
    """
    return r.is_epaxos() and isinstance(r.workload(),
        Experiment.ZipfianWorkload)

def get_epaxos_zipf_result(results):
    """
    Returns a Results object from the 'results' list in which the protocol was
    EPaxos and the workload was Zipfian.
    """
    return list(filter(is_epaxos_zipf_result, results))

def get_mpaxos_result(results):
    """
    Returns a Results object from the 'results' list in which the protocol was
    Multi-Paxos.
    """
    return list(filter(lambda r: r.is_mpaxos(), results))

def legend_line(color, linestyle='-'):
    """
    Returns a line with color 'color' and linestyle 'linestyle' that is
    appropriate for use in a legend.
    """
    return lines.Line2D([0], [0], color=color, lw=4, linestyle=linestyle)

def legend_patch_color(color):
    """
    Returns a rectangular swatch with fill color 'color' and a black outline
    that is appropriate for use in a legend.
    """
    return patches.Patch(facecolor=color, edgecolor='black')

def legend_patch_hatch(hatch=None):
    """
    Returns a rectangular swatch that is appropriate for use in a legend.
    'hatch' refers to the fill pattern of the swatch. If there is no hatch, the
    swatch is a solid black rectangle. If there is a hatch, the swatch is a
    white rectangle with black outline and the hatch as the fill pattern.
    """
    return patches.Patch(edgecolor='black',
        facecolor='black' if hatch is None else 'white', hatch=hatch)

def get_color(result):
    """
    Returns the consistent color to associate with a given kind of experiment.
    These colors associate with those used in the original EPaxos evaluation for
    easy comparison.
    """
    if result.batching_enabled() or result.thrifty():
        return ALTERNATE_ZIPF_COLOR
    if result.is_mpaxos(): return '#0085fa' # Blue
    if is_fixed_epaxos_result(result, 0): return '#FFD700' # Yellow
    if is_fixed_epaxos_result(result, 2): return '#FF7F00' # Orange
    if is_fixed_epaxos_result(result, 100): return '#B0171F' # Red
    if result.is_epaxos() and isinstance(result.workload(),
        Experiment.ZipfianWorkload): return 'purple'

def plot_cdf(ax, data, color, linestyle='-'):
    """
    Plots a inverse cumulative distribution of 'data' on the axes 'ax'. The line
    is plotted in the provided 'color' with the provided 'linestyle'.
    """
    n, bins, patches = ax.hist(data, bins=100, density=True, histtype='step',
        cumulative=-1, color=color, zorder=10, linewidth=2, linestyle=linestyle)
    patches[0].set_xy(patches[0].get_xy()[1:])

def format_cdf(ax):
    """
    Styles an axes that contains a cdf plot. Cdfs are logscaled, with markers
    on the y-axis for each power of 10 between .01% and 100%, inclusive. The
    y-axis markers are formatted as percentages. A grey grid is plotted at the
    x and y ticks.
    """
    ax.set_yscale('log')
    ax.set_xlim(xmin=0)
    ax.set_ylim(ymax=1)
    ax.set_yticks([.0001, .001, .01, .1, 1])
    ax.grid(which='major', color='#a3a3a3', linewidth='0.5')

    def to_percent(y, position):
        # Ignore the passed in position. This has the effect of scaling the default
        # tick locations.
        s = str(100 * y)
        if s.endswith('.0'): s = s[:-2]
        if s.startswith('0'): s = s[1:]

        # The percent symbol needs escaping in latex
        if matplotlib.rcParams['text.usetex'] is True:
            return s + r'$\%$'
        else:
            return s + '%'
    ax.yaxis.set_major_formatter(ticker.FuncFormatter(to_percent))

def make_legend(ax, handles, labels, ncol, loc, size=20, squeeze=False, bbox_to_anchor=None):
    """
    Plots and returns a legend on the Axes 'ax'. 'handles' contains the swatches
    for the legend, and 'labels' contains the text to associate with each
    handle. (labels[i] corresponds to the text for handles[i].) 'ncol' refers to
    the number of columns for the legend. 'loc' refers to the location of the
    legend. 'size' refers to the font size for the legend, and handles are
    scaled accordingly. If 'squeeze' is True, the margins between the handles/
    text and the border of the legend are made smaller.
    """
    return ax.legend(handles, labels, facecolor='white',  ncol=ncol, loc=loc,
        columnspacing=1, prop={'size': size},
        borderaxespad=.1 if squeeze else .3, borderpad=.3 if squeeze else .5,
        handletextpad=.5, bbox_to_anchor=bbox_to_anchor)

def plot_bar_by_loc(ax, expts, yfn, errfn, colorfn, annotatefn, barwidth,
    fontsize, maxy, ystep, annotationsize=11, locs=ORDERED_LOCS,
    hatches=[None, '//', None], hatch_fill=[0, 1], annotationangle=45,
    errwidth=10, actual_barwidth=None):
    """
    Plots a bar graph on the Axes 'ax'. The bar graph is the same format as
    Figure 4 from the original EPaxos paper; for each location, the results
    from each experiment in 'expts' are side by side, and the locations are
    separated by whitespace. 'yfn' is a function that determines the y values
    that should be plotted given (expt, loc) arguments. The function can
    return any number of y values to plot, which will be plotted consecutively
    behind one another with different hatches, so the returned values should be
    in increasing order. 'errorfn' is an optional function that takes in
    (expt, loc) parameters and returns the y value for an error bar above the
    plotted bar for that experiment and location. 'colorfn' is a function that
    takes in an experiment and returns the color that its bars should be.
    'annotatefn' is a function that takes in an (expt, loc) and returns a string
    to be placed above the topmost bar or error assocated with that experiment
    and location. 'barwidth' refers to the horizontal dimension of the plotted
    bars. 'fontsize' refers to the size of the x and y labels. 'maxy' refers to
    the y value of the top of the graph. 'ystep' refers to the intervals at
    which y markers should be placed. 'annotationsize' refers to the font size
    of annotations. 'annotationheight' refers to how far above the bar the
    annotation should be. 'annotationhadjust' is a function that takes in the
    index of the experiment and returns an amount to move the annotation in
    the horizontal direction. 'xlabelhadjust' refers to the amount the location
    labels should be moved left of center in order for them to appear centered.
    """
    xpos = np.arange(len(locs))

    if not actual_barwidth: actual_barwidth = barwidth

    for loci, loc in enumerate(locs):
        maxerr = 0

        for expti, expt in enumerate(expts):
            x = xpos[loci] + expti*barwidth

            ys = yfn(expt, loc)
            errs = errfn(expt, loc) if errfn is not None else [None for _ in ys]

            # Plot each y value in ascending order. The first will be solidly
            # filled, and the remaining will be white with different hatch
            # fillings.
            for yi, y in enumerate(ys):
                color = colorfn(expt) if yi in hatch_fill else 'white'
                edgecolor = 'black' if yi in hatch_fill else colorfn(expt)

                yerr = errs[yi]
                if yerr is not None:
                    err_min, err_max = yerr
                    yerr = [[y-err_min], [err_max-y]]
                    if yi != len(ys)-1: lower = err_max
                    maxerr = max(maxerr, err_max)
                rect = ax.bar(x, y, actual_barwidth, color=color, edgecolor=edgecolor,
                    hatch=hatches[yi], zorder=10-yi, yerr=yerr, capsize=errwidth)

        for expti, expt in enumerate(expts):
            # Add annotation at the bottom of the bar if appropriate
            if annotatefn is not None:
                annotation = annotatefn(expt, loc)
                if annotation is not None:
                    # x = rect[0].get_x()+rect[0].get_width()/2.
                    # y = rect[0].get_y()+lower+(err_min-lower)/2 # adjustment
                    # ax.text(x, y, annotation, ha='center',
                    #     va='center', size=annotationsize, zorder=10, color='black', rotation=270)
                    # x = rect[0].get_x()+rect[0].get_width()/2.
                    x = xpos[loci] + expti*barwidth - barwidth/4.
                    y = maxerr + 4 # adjustment
                    # ax.text(x, y, annotation, ha='left', bbox=dict(facecolor='white', edgecolor=colorfn(expt), boxstyle='round'),
                    #     va='bottom', size=annotationsize, zorder=10, color='black', rotation=45)
                    ax.text(x, y, annotation, ha='left', bbox=dict(facecolor='white', edgecolor='white', pad=0),
                        va='bottom', size=annotationsize+4, zorder=4, color=colorfn(expt), rotation=annotationangle)

    # Add labels for locations on the x axis and set font size for y axis
    # labels.

    if len(locs) > 1:
        ax.set_xticks(xpos+barwidth*len(expts)/2.-barwidth/2.)
        ax.set_xticklabels([l.upper() for l in locs], fontsize=fontsize, ha='center')
    else: ax.set_xticklabels('')
    ax.tick_params(axis='x', length=0)
    ax.tick_params(axis='y', labelsize=fontsize)


    # Add horizontal lines at specified y intervals.
    ax.set_ylim(0, maxy)
    ys = np.arange(0, maxy+1, ystep)
    ax.set_yticks(ys)
    for y in ys[1:-1]:
        ax.axhline(y=y, color='gray', linestyle='dotted', zorder=1)
    if ys[-1] != maxy:
        ax.axhline(y=ys[-1], color='gray', linestyle='dotted', zorder=1)

def conflict_annotation(expt, loc):
    """
    Returns conflict rate of an experiment as a string percentage. Only does
    this if the experiment is EPaxos, as MPaxos has no conflict rate.
    """
    if expt.is_epaxos() and not is_fixed_epaxos_result(expt, 0):
        conflict_rate = '{}%'.format(round(expt.conflict_rate(loc)*100, 1))
        return conflict_rate

    return None

def reproduction_bar(dirname):
    """
    Generates a graph that reproduces the results of the original EPaxos paper,
    comparing commit and execution latencies, with conflict rate. 'dirname' is a
    directory containing experiments for EPaxos 0%, EPaxos 2%, EPaxos 100%,
    EPaxos Zipf, and MPaxos. The generated graph is saved as an image in the
    directory specified by 'dirname'.
    """
    plt.clf()

    results = get_results(dirname)
    epaxos_0 = get_fixed_epaxos_result(results, 0)
    epaxos_2 = get_fixed_epaxos_result(results, 2)
    epaxos_100 = get_fixed_epaxos_result(results, 100)
    epaxos_zipf = get_epaxos_zipf_result(results)
    mpaxos = get_mpaxos_result(results)
    expts = [epaxos_0, epaxos_2, epaxos_100, epaxos_zipf, mpaxos]

    fig, axs = plt.subplots(2, 1, figsize=(13, 6), constrained_layout=True)
    barwidth = .18
    fontsize = 16
    maxy = 400
    ystep = 100

    color_fn = lambda expts: get_color(expts[0])

    # Plot mean commit latency vs. mean execution latency on the top graph
    plot_bar_by_loc(axs[0], expts, lambda expts, loc: ([statistics.mean([expt.mean_lat_commit(loc) for expt in expts]),
        statistics.mean([expt.mean_lat_exec(loc) for expt in expts])]),
        lambda expts, loc: [(min([expt.mean_lat_commit(loc) for expt in expts]),
            max([expt.mean_lat_commit(loc) for expt in expts])), (min([expt.mean_lat_exec(loc) for expt in expts]),
            max([expt.mean_lat_exec(loc) for expt in expts]))],
        color_fn, None,#conflict_range_annotation,
        barwidth, fontsize, maxy, ystep,
        hatches=[None, '//'], hatch_fill=[0], errwidth=6)
    # Plot 99th percentile commit latency vs. 99th percentile execution latency
    # on the bottom graph
    plot_bar_by_loc(axs[1], expts, lambda expts, loc: ([statistics.mean([expt.p99_lat_commit(loc) for expt in expts]),
        statistics.mean([expt.p99_lat_exec(loc) for expt in expts])]),
        lambda expts, loc: [(min([expt.p99_lat_commit(loc) for expt in expts]),
            max([expt.p99_lat_commit(loc) for expt in expts])), (min([expt.p99_lat_exec(loc) for expt in expts]),
            max([expt.p99_lat_exec(loc) for expt in expts]))],
        color_fn, conflict_range_annotation, barwidth,
        fontsize, maxy, ystep,
        hatches=[None, '//'], hatch_fill=[0], annotationangle=27, errwidth=6)

    axs[1].annotate('Conflict Rate', xy=(.34, 340), xytext=(.19, 340),
        fontsize=14, arrowprops=dict(facecolor='black', arrowstyle='-|>'),
        verticalalignment='center', horizontalalignment='right')
    # axs[1].annotate('Conflict Rate', xy=(.17, 280), xytext=(.2, 230),
    #     fontsize=14, arrowprops=dict(facecolor='black', arrowstyle='-|>'),
    #     verticalalignment='center', horizontalalignment='right')
    # axs[1].annotate(' ', xy=(1.6, 290), xytext=(1.75, 275),
    #     fontsize=14, arrowprops=dict(facecolor='black', arrowstyle='-|>'),
    #     verticalalignment='top')
    # axs[1].annotate('Conflict Rate', xy=(2.15, 290), xytext=(2.11, 250),
    #     fontsize=14, arrowprops=dict(facecolor='black', arrowstyle='-|>'),
    #     verticalalignment='center', horizontalalignment='right')

    axs[0].set_ylabel('Mean Latency (ms)', fontsize=fontsize)
    axs[1].set_ylabel('P99 Latency (ms)', fontsize=fontsize)

    # Add two legends to the top graph: on the left, the legend tells us which
    # colors refer to which experiments. On the right, the legend tells us
    # which fill patterns refer to commit and execution latency.
    leg = make_legend(axs[0], [legend_patch_color(color_fn(e)) for e in expts],
        [e[0].description() for e in expts], ncol=5, loc='upper left', size=fontsize)
    make_legend(axs[0], [legend_patch_hatch(HATCHES[i]) for i in range(2)],
        ['Commit', 'Exec'], ncol=2, loc='upper right', size=fontsize)
    axs[0].add_artist(leg)

    plt.savefig(path.join(dirname, 'reproduction_bar.pdf'))

def batching_bar(dirname):
    """
    Generates a graph that compares the execution latency between EPaxos with
    batching, EPaxos without batching, and MPaxos. 'dirname' is a directory
    containing those three experiments. The generated graph is saved as an image
    in the directory specified by 'dirname'.
    """
    plt.clf()

    results = get_results(dirname)
    batching = list(filter(lambda r: r.is_epaxos() and r.batching_enabled(),
        results))
    no_batching = list(filter(lambda r: r.is_epaxos() and not r.batching_enabled(),
        results))
    mpaxos = get_mpaxos_result(results)
    expts = [batching, no_batching, mpaxos]

    fig, ax = plt.subplots(figsize=(13, 4), constrained_layout=True)
    barwidth = .25
    fontsize = 22
    maxy = 500
    ystep = 100

    # Plots mean latency as a bar and 99th percentile latency as an error bar
    # above it.
    plot_bar_by_loc(ax, expts, lambda expt, loc: [statistics.mean([e.mean_lat_exec(loc) for e in expt]),
        statistics.mean([e.p99_lat_exec(loc) for e in expt])],
        lambda expt, loc: [[min([e.mean_lat_exec(loc) for e in expt]), max([e.mean_lat_exec(loc) for e in expt])],
        [min([e.p99_lat_exec(loc) for e in expt]), max([e.p99_lat_exec(loc) for e in expt])]],
        lambda e: get_color(e[0]),
        conflict_range_annotation, barwidth, fontsize, maxy, ystep, annotationsize=16,#20,
        # annotationheight=0.6,
        # Move the conflict rate labels slightly to the left and right so that
        # they are easier to read and can be larger without overlapping.
        # annotationhadjust=lambda expti: (-0.05 if expti == 0 else .05),
        # xlabelhadjust=.12
        # extra_err_fn=lambda expt, loc: [min([e.p99_lat_exec(loc) for e in expt]), max([e.p99_lat_exec(loc) for e in expt])],
        hatches=[None, None], hatch_fill=[0], annotationangle=27,
        )

    ax.set_ylabel('Mean/P99 Latency (ms)', fontsize=fontsize)

    leg = make_legend(ax, [legend_patch_color(get_color(e[0])) for e in expts],
        ['Batching', 'No Batching', mpaxos[0].description()], ncol=3,
        loc='lower right', size=fontsize, squeeze=True)
    leg.set_zorder(20)

    plt.savefig(path.join(dirname, 'batching_bar.pdf'))

def base_latency(expt, loc):
    if isinstance(expt, list): expt = expt[0]
    if expt.is_epaxos():
        if expt.clock_sync_str() in [CLOCK_SYNC_NONE, CLOCK_SYNC_QUORUM]:
            return {
                'ca': 64,
                'va': 64,
                'or': 59,
                'jp': 98,
                'eu': 129,
            }[loc]
        if expt.clock_sync_str() == CLOCK_SYNC_QUORUM_UNION:
            return {
                'ca': 64,
                'va': 64,
                'or': 59,
                'jp': 98 + (73-49),
                'eu': 129 + (68-64),
            }[loc]
        if expt.clock_sync_str() == CLOCK_SYNC_CLUSTER:
            return {
                'ca': 64 + (68-32),
                'va': 64 + (73-32),
                'or': 59 + (64-29),
                'jp': 98 + (110-49),
                'eu': 129 + (110-64),
            }[loc]

    if expt.is_mpaxos():
        return {
            'ca': 64,
            'va': 128,
            'or': 90,
            'jp': 162,
            'eu': 201,
        }[loc]

def osc_bar(dirname):
    """
    TODO(sktollman): add comment
    """
    plt.clf()

    results = get_results(dirname)

    workloads = [
        (.8, .5),
        (.99, 1),
        # (.9, .5),
    ]

    fig, axs = plt.subplots(len(workloads), 1, figsize=(22, 10*len(workloads)), constrained_layout=True)
    barwidth = .15
    fontsize = 24
    maxy = 400
    ystep = 100

    def color_fn(r):
        if r[0].clock_sync_str() == CLOCK_SYNC_QUORUM:
            return '#FF7F00'#FFD700'#'black'
        if r[0].clock_sync_str() == CLOCK_SYNC_QUORUM_UNION:
            return 'green'
        if r[0].clock_sync_str() == CLOCK_SYNC_CLUSTER:
            return 'red'

        return get_color(r[0])

    for i, (theta, writes) in enumerate(workloads):
        ax = axs[i]

        none = list(filter(lambda r: r.is_epaxos() and r.clock_sync_str() == CLOCK_SYNC_NONE and \
            r.frac_writes() == writes and r.theta() == theta,
            results))
        quorum = list(filter(lambda r: r.is_epaxos() and r.clock_sync_str() == CLOCK_SYNC_QUORUM and \
            r.frac_writes() == writes and r.theta() == theta,
            results))
        quorum_union = list(filter(lambda r: r.is_epaxos() and r.clock_sync_str() == CLOCK_SYNC_QUORUM_UNION and \
            r.frac_writes() == writes and r.theta() == theta,
            results))
        cluster = list(filter(lambda r: r.is_epaxos() and r.clock_sync_str() == CLOCK_SYNC_CLUSTER and \
            r.frac_writes() == writes and r.theta() == theta,
            results))
        mpaxos = get_mpaxos_result(results)
        expts = [*[cluster, quorum_union, quorum, none][::-1], mpaxos]

        # for loc in ORDERED_LOCS:
        #     # mean_reg = statistics.mean([e.mean_lat_exec(loc) for e in none])
        #     # dec = lambda x: round(100*(mean_reg-statistics.mean([e.mean_lat_exec(loc) for e in x]))/mean_reg,2)

        #     conflict_reg = statistics.mean([e.conflict_rate(loc) for e in none])
        #     dec = lambda x: '{}%'.format(round(100*(conflict_reg-statistics.mean([e.conflict_rate(loc) for e in x]))/conflict_reg,2))

        #     print(theta, writes, loc, "Quorum dec:", dec(quorum))
        #     print(theta, writes, loc, "Quorum union dec:", dec(quorum_union))

        # conflict_reg = sum([statistics.mean([e.conflict_rate(loc) for e in none]) for loc in ORDERED_LOCS])
        # dec = lambda x: '{}%'.format(round(100*(conflict_reg-sum([statistics.mean([e.conflict_rate(loc) for e in x]) for loc in ORDERED_LOCS]))/conflict_reg,2))

        # print(theta, writes, "TOTAL Quorum dec:", dec(quorum))
        # print(theta, writes, "TOTAL  Quorum union dec:", dec(quorum_union))

        mean_reg = sum([statistics.mean([e.mean_lat_exec(loc) for e in none]) for loc in ORDERED_LOCS])
        dec = lambda x: '{}%'.format(round(100*(mean_reg-sum([statistics.mean([e.mean_lat_exec(loc) for e in x]) for loc in ORDERED_LOCS]))/mean_reg,2))

        print(theta, writes, "TOTAL Quorum dec:", dec(quorum))
        print(theta, writes, "TOTAL  Quorum union dec:", dec(quorum_union))

        # Plots mean latency as a bar and 99th percentile latency as an error bar
        # above it.
        plot_bar_by_loc(ax, expts, lambda expt, loc: [base_latency(expt[0], loc),
            # expt.mean_lat_commit(loc),
            statistics.mean([e.mean_lat_exec(loc) for e in expt]),
            statistics.mean([e.p99_lat_exec(loc) for e in expt])],
            # lambda expt, loc: expt.p99_lat_exec(loc),
            lambda expt, loc: [None,
                (min([e.mean_lat_exec(loc) for e in expt]), max([e.mean_lat_exec(loc) for e in expt])),
                (min([e.p99_lat_exec(loc) for e in expt]), max([e.p99_lat_exec(loc) for e in expt]))
            ],
            color_fn,
            conflict_range_annotation, barwidth, fontsize, maxy, ystep, annotationsize=20, annotationangle=40,
            # Move the conflict rate labels slightly to the left and right so that
            # they are easier to read and can be larger without overlapping.
            # extra_err_fn=lambda expt, loc: (min([e.p99_lat_exec(loc) for e in expt]), max([e.p99_lat_exec(loc) for e in expt]))
            )

        ax.set_ylabel('Latency (ms)', fontsize=fontsize)
        ax.set_title('Zipfian Skew (θ): {}\n% Writes: {}%'.format(theta, 100*writes), fontsize=fontsize)

    leg = make_legend(axs[0], [legend_patch_color(color_fn(e)) for e in expts],
        [*['All', 'Quorum Union', 'Quorum', 'No TOQ'][::-1], mpaxos[0].description()], ncol=6,
        size=fontsize, loc='upper left')

    # leg = make_legend(axs[0], [legend_patch_color(get_color(e)) for e in expts],
    #     [e.description() for e in expts], ncol=5, loc='upper left')
    make_legend(axs[0], [legend_patch_hatch(HATCHES[i]) for i in range(2)] +
        [patches.Patch(edgecolor='black', facecolor='white')],
        ['Minimum', 'Mean Exec', 'P99 Exec'], ncol=2, loc='upper right')
    axs[0].add_artist(leg)

    plt.savefig(path.join(dirname, 'osc_bar.pdf'))

def osc_bar_loc(dirname):
    """
    TODO
    """
    loc = 'or'

    plt.clf()

    results = get_results(dirname)

    workloads = [
        # (.8, .5),
        (.99, 1),
        # (.9, .5),
    ]

    fig, ax = plt.subplots(1, len(workloads), figsize=(7, 15), constrained_layout=True)
    barwidth = .15
    fontsize = 32
    maxy = 500
    ystep = 100

    def color_fn(r):
        if r[0].clock_sync_str() == CLOCK_SYNC_QUORUM:
            return '#FF7F00'#FFD700'#'black'
        if r[0].clock_sync_str() == CLOCK_SYNC_QUORUM_UNION:
            return 'green'
        if r[0].clock_sync_str() == CLOCK_SYNC_CLUSTER:
            return 'red'

        return get_color(r[0])

    for i, (theta, writes) in enumerate(workloads):
        # ax = axs[i]

        none = list(filter(lambda r: r.is_epaxos() and r.clock_sync_str() == CLOCK_SYNC_NONE and \
            r.frac_writes() == writes and r.theta() == theta,
            results))
        quorum = list(filter(lambda r: r.is_epaxos() and r.clock_sync_str() == CLOCK_SYNC_QUORUM and \
            r.frac_writes() == writes and r.theta() == theta,
            results))
        quorum_union = list(filter(lambda r: r.is_epaxos() and r.clock_sync_str() == CLOCK_SYNC_QUORUM_UNION and \
            r.frac_writes() == writes and r.theta() == theta,
            results))
        cluster = list(filter(lambda r: r.is_epaxos() and r.clock_sync_str() == CLOCK_SYNC_CLUSTER and \
            r.frac_writes() == writes and r.theta() == theta,
            results))
        mpaxos = get_mpaxos_result(results)
        expts = [none, quorum, quorum_union, cluster, mpaxos]

        # Plots mean latency as a bar and 99th percentile latency as an error bar
        # above it.
        plot_bar_by_loc(ax, expts, lambda expt, loc: [base_latency(expt[0], loc),
            # expt.mean_lat_commit(loc),
            statistics.mean([e.mean_lat_exec(loc) for e in expt]),
            statistics.mean([e.p99_lat_exec(loc) for e in expt])],
            # lambda expt, loc: expt.p99_lat_exec(loc),
            lambda expt, loc: [None,
                (min([e.mean_lat_exec(loc) for e in expt]), max([e.mean_lat_exec(loc) for e in expt])),
                (min([e.p99_lat_exec(loc) for e in expt]), max([e.p99_lat_exec(loc) for e in expt]))
            ],
            color_fn,
            conflict_range_annotation, barwidth, fontsize, maxy, ystep, annotationsize=28,
            locs=['or'], annotationangle=44, actual_barwidth=.08,
            # Move the conflict rate labels slightly to the left and right so that
            # they are easier to read and can be larger without overlapping.
            # extra_err_fn=lambda expt, loc: (min([e.p99_lat_exec(loc) for e in expt]), max([e.p99_lat_exec(loc) for e in expt]))
            )

        ax.set_ylabel('Latency (ms)', fontsize=fontsize)
        # ax.set_title('Zipfian Skew (θ): {}\n% Writes: {}%'.format(theta, 100*writes), fontsize=fontsize)

    # leg = make_legend(ax, [legend_patch_color(color_fn(e)) for e in expts],
    #     ['No OSC', 'Quorum', 'Quorum Union', 'All', mpaxos[0].description()], ncol=1,
    #     size=28, loc='upper right', bbox_to_anchor=(0.45, 1.27), squeeze=True)
    # leg.set_zorder(20)

    # leg = make_legend(axs[0], [legend_patch_color(get_color(e)) for e in expts],
    #     [e.description() for e in expts], ncol=5, loc='upper left')
    leg2 = make_legend(ax,
        [legend_patch_hatch(HATCHES[i]) for i in range(2)] + [patches.Patch(edgecolor='black', facecolor='white')],
        ['Minimum', 'Mean', 'P99'], ncol=2, loc='upper left', size=30, #squeeze=True,
        # bbox_to_anchor=(0.48, 1.27)
        )
    leg2.set_zorder(20)

    ax.set_xticks([barwidth*i+.04 for i, _ in enumerate(expts)])

    ax.set_xticklabels(['No OSC', 'Quorum', 'Quorum Union', 'All', mpaxos[0].description()],
        fontsize=fontsize, ha='right', rotation=35)
    # ax.add_artist(leg)

    plt.savefig(path.join(dirname, 'osc_bar_{}.pdf'.format(loc)))

def conflict_range_annotation(es, loc):
    if es[0].is_mpaxos(): return ''
    if is_fixed_epaxos_result(es[0], 0): return ''
    conflicts = [e.conflict_rate(loc)*100 for e in es]
    smallest = round(min(conflicts),1)
    biggest = round(max(conflicts),1)
    # smallest = trim_zeros(smallest)
    # biggest = trim_zeros(biggest)
    if smallest == biggest:
        return '{}%'.format(smallest)
    # return '{}—{}%'.format(smallest, biggest)
    return '{} – {}%'.format(smallest, biggest)

def trim_zeros(x):
    x = '{}'.format(x)
    if x.endswith('.0'): x = x[:-2]
    if x.startswith('0') and len(x) > 1: x = x[1:]
    return x

def conflict_stdev_annotation(es, loc):
    if es[0].is_mpaxos(): return ''
    if is_fixed_epaxos_result(es[0], 0): return ''
    conflicts = [e.conflict_rate(loc)*100 for e in es]
    mean = '{}'.format(round(statistics.mean(conflicts),1))
    stdev = '{}'.format(round(statistics.stdev(conflicts),1))
    # mean = trim_zeros(mean)
    # stdev = trim_zeros(stdev)
    if float(stdev) == 0:
        return '{}%'.format(mean)
    return '{}% (±{}%)'.format(mean, stdev)

def thrifty_bar(dirname):
    """
    TODO
    """
    plt.clf()

    results = get_results(dirname)
    thrifty = list(filter(lambda r: r.is_epaxos() and r.thrifty(),
        results))
    no_thrifty = list(filter(lambda r: r.is_epaxos() and not r.thrifty(),
        results))

    workloads = [
        (.7, .3),
        (.9, .5),
        (.99, 1)
    ]

    fig, axs = plt.subplots(len(workloads), 1, figsize=(9, 5*len(workloads)+len(workloads)-1),
        constrained_layout=True)
    barwidth = .3
    # fig, axs = plt.subplots(1, len(workloads), figsize=(5*len(workloads)+len(workloads)-1, 9),
    #     constrained_layout=True)
    # barwidth = .2
    fontsize = 22

    rows = []

    for i, (theta, writes) in enumerate(workloads):
        ax = axs[i]

        # TODO: multiple trials!
        thrifty = list(filter(lambda r: r.is_epaxos() and r.thrifty() and \
            r.theta() == theta and r.frac_writes() == writes, results))
        no_thrifty = list(filter(lambda r: r.is_epaxos() and not r.thrifty() and \
            r.theta() == theta and r.frac_writes() == writes, results))

        expts = [thrifty, no_thrifty]

        for e in thrifty:
            if i == 2:
                print(e.mean_lat_exec('ca'))

        # Plots mean latency as a bar and 99th percentile latency as an error bar
        # above it.
        maxy = 500 #if i != 2 else 7000
        ystep = 100 #if i != 2 else 1000
        plot_bar_by_loc(ax, expts, lambda expts, loc: [statistics.mean(expt.mean_lat_exec(loc) for expt in expts), statistics.mean(expt.p99_lat_exec(loc) for expt in expts)],
            # lambda expt, loc: expt.p99_lat_exec(loc),
            lambda expts, loc: [(min([e.mean_lat_exec(loc) for e in expts]), max([e.mean_lat_exec(loc) for e in expts])),
                (min([e.p99_lat_exec(loc) for e in expts]), max([e.p99_lat_exec(loc) for e in expts]))],
            lambda expts: get_color(expts[0]),
            conflict_range_annotation, barwidth, fontsize, maxy, ystep, annotationsize=16,
            hatches=[None, None], hatch_fill=[0], annotationangle=40,
            # annotationheight=0.6,
            # Move the conflict rate labels slightly to the left and right so that
            # they are easier to read and can be larger without overlapping.
            # annotationhadjust=lambda expti: (-0.05 if expti == 0 else .05),
            # xlabelhadjust=.12,
            # extra_err_fn=lambda expts, loc: [min([expt.p99_lat_exec(loc) for expt in expts]),
            #     max([expt.p99_lat_exec(loc) for expt in expts])]
            )

        ax.set_ylabel('Mean/P99 Latency (ms)', fontsize=fontsize)
        ax.set_title('Zipfian Skew (θ): {}\n% Writes: {}%'.format(theta, 100*writes), fontsize=fontsize)

        thrifty = list(filter(lambda r: r.is_epaxos() and r.thrifty() and \
            r.theta() == theta and r.frac_writes() == writes, results))
        no_thrifty = list(filter(lambda r: r.is_epaxos() and not r.thrifty() and \
            r.theta() == theta and r.frac_writes() == writes, results))

        for t, expts in [(False, no_thrifty), (True, thrifty)]:
            for loc in ORDERED_LOCS:
                conflicts = [expt.conflict_rate(loc)*100 for expt in expts]

                conflict_str = '{}%'.format(round(statistics.mean(conflicts), 2))
                if t:
                    no_t_conflicts = [expt.conflict_rate(loc)*100 for expt in no_thrifty]
                    conflict_str += ' (+{}%)'.format(round(100*(statistics.mean(conflicts)-statistics.mean(no_t_conflicts))/statistics.mean(no_t_conflicts), 2))

                means = [expt.mean_lat_exec(loc) for expt in expts]
                p99s = [expt.p99_lat_exec(loc) for expt in expts]

                mean_str = str(round(statistics.mean(means),2))
                if t:
                    no_t_means = [expt.mean_lat_exec(loc) for expt in no_thrifty]
                    mean_str += ' (+{}%)'.format(round(100*(statistics.mean(means)-statistics.mean(no_t_means))/statistics.mean(no_t_means), 2))

                p99_str = str(round(statistics.mean(p99s),2))
                if t:
                    no_t_p99s = [expt.p99_lat_exec(loc) for expt in no_thrifty]
                    p99_str += ' (+{}%)'.format(round(100*(statistics.mean(p99s)-statistics.mean(no_t_p99s))/statistics.mean(no_t_p99s), 2))

                rows.append((writes, theta, t, loc,
                    conflict_str,
                    '{}% ({}%)'.format(round(statistics.stdev(conflicts), 2),
                        round(statistics.stdev(conflicts)/statistics.mean(conflicts)*100, 2)),
                    mean_str,
                    round(statistics.stdev(means),2),
                    p99_str,
                    round(statistics.stdev(p99s),2),
                ))

            rows.append(())

        expts = [thrifty[0], no_thrifty[0]]

    make_legend(axs[0], [legend_patch_color(get_color(e)) for e in expts],
        ['Thrifty', 'No Thrifty'], ncol=3,
        loc='upper right', size=fontsize)

    plt.savefig(path.join(dirname, 'thrifty_bar.pdf'))

    with open(path.join(dirname, 'thrifty_stats.txt'), 'w') as f:
        print(tabulate(rows, headers=[
            'Frac Writes', 'Theta', 'Thrifty', 'Location', 'Conflict Avg',
            'Conflict Stdev', 'Mean Lat Avg', 'Mean Lat Stdev', 'P99 Lat Avg',
            'P99 Lat Stdev'], tablefmt='github'), file=f)

def commitvexec_cdf(dirname, loc='or'):
    """
    Generates a CDF graph that compares the commit and execution latency of
    EPaxos. The graph contains arrows indicating that the fast path is 1 RTT,
    the slow path (worst case for commit latency) is 2 RTTs, and execution
    latency is bounded, but can be much worse than worst case commit latency.
    'dirname' is a directory containing an EPaxos experiment with Zipfian
    workload. The generated graph is saved as an image in the directory
    specified by 'dirname'. 'loc' specifies which client location's latency
    should be plotted; all client locations will show the same patterns, so we
    only plot one.
    """
    plt.clf()

    results = get_results(dirname)
    expt = get_epaxos_zipf_result(results)[0]
    mpaxos = get_mpaxos_result(results)[0]

    fig, ax = plt.subplots(figsize=(7, 4), constrained_layout=True)

    # Add CDF lines for commit and exec latency.
    plot_cdf(ax, mpaxos.all_lats_exec(loc), get_color(mpaxos))
    plot_cdf(ax, expt.all_lats_commit(loc), ALTERNATE_ZIPF_COLOR, '--')
    plot_cdf(ax, expt.all_lats_exec(loc), get_color(expt))

    # Add arrows highlighting key latencies.
    make_arrow = lambda text, x: ax.annotate(text, xy=(x, 1), xytext=(x, 10),
        fontsize=18, arrowprops=dict(facecolor='black', arrowstyle='-|>'),
        verticalalignment='top', horizontalalignment='center')
    make_arrow('1 RTT', expt.p50_lat_exec(loc))
    make_arrow('2 RTTs', expt.p99_lat_commit(loc))
    make_arrow('Bound', sorted(expt.all_lats_exec(loc))[-1])

    print('1 RTT', expt.p50_lat_exec(loc))
    print('2 RTTs', expt.p99_lat_commit(loc))
    print('Bound', sorted(expt.all_lats_exec(loc))[-1])
    print('MPaxos', mpaxos.p50_lat_exec(loc))

    format_cdf(ax)

    make_legend(ax, [legend_line(get_color(expt)),
        legend_line(ALTERNATE_ZIPF_COLOR, '--'),
        legend_line(get_color(mpaxos))], ['Exec', 'Commit', mpaxos.description()],
        ncol=1, loc='lower left', size=22, squeeze=True,
        bbox_to_anchor=(.5, 0))

    ax.set_xlabel('Latency (ms)', fontsize=20)
    ax.set_ylabel('% Operations', fontsize=20)
    ax.tick_params(axis='both', labelsize=18)

    plt.savefig(path.join(dirname, 'commitvexec_cdf_{}.pdf'.format(loc)))

def infinite_cdf(dirname, loc='or'):
    """
    Generates a CDF graph that compares the execution latency of EPaxos with and
    without a modification that bounds execution delay. 'dirname' is a directory
    containing two EPaxos experiments with Zipfian workload, one with the fix
    and one without. The generated graph is saved as an image in the directory
    specified by 'dirname'. 'loc' specifies which client location's latency
    should be plotted; all client locations will show the same patterns, so we
    only plot one.
    """
    plt.clf()

    fontsize = 24

    results = get_results(dirname)
    inffix = list(filter(lambda r: is_epaxos_zipf_result(r) and r.inffix(),
        results))[0]
    no_inffixs = list(filter(lambda r: is_epaxos_zipf_result(r) and not r.inffix(),
        results))

    for no_inffix in no_inffixs:
        fig, ax = plt.subplots(figsize=(6, 4), constrained_layout=True)

        plot_cdf(ax, inffix.all_lats_exec(loc), get_color(inffix))
        plot_cdf(ax, no_inffix.all_lats_exec(loc), ALTERNATE_ZIPF_COLOR, '--')

        format_cdf(ax)

        make_legend(ax, [legend_line(get_color(inffix)),
            legend_line(ALTERNATE_ZIPF_COLOR, '--')][::-1], ['Improved', 'Unmodified'][::-1],
            ncol=1, loc='lower center', size=fontsize, squeeze=True)

        ax.set_xlabel('Latency (ms)', fontsize=fontsize)
        ax.set_ylabel('% Operations', fontsize=fontsize)
        # ax.set_xticks([0, 500, 1000, 1500, 2000])
        # ax.set_xticks([0, 1250, 2500, 3750, 5000])
        ax.set_xticks([0, 500, 1000, 1500, 2000, 2500])
        ax.tick_params(axis='both', labelsize=fontsize)

        plt.savefig(path.join(dirname, 'infinite_cdf_{}_{}.pdf'.format(loc, no_inffix.arrival_rate())))

def or_vs_psn_cdf(dirname, loc='or'):
    """
    Generates a CDF graph that compares the execution latency of EPaxos with and
    without a modification that bounds execution delay. 'dirname' is a directory
    containing two EPaxos experiments with Zipfian workload, one with the fix
    and one without. The generated graph is saved as an image in the directory
    specified by 'dirname'. 'loc' specifies which client location's latency
    should be plotted; all client locations will show the same patterns, so we
    only plot one.
    """
    plt.clf()

    fontsize = 24

    results = get_results(dirname)
    psn = list(filter(lambda r: is_epaxos_zipf_result(r) and isinstance(r.arrival_rate(), Experiment.PoissonArrivalRate),
        results))[0]
    ors = list(filter(lambda r: is_epaxos_zipf_result(r) and isinstance(r.arrival_rate(), Experiment.OutstandingReqArrivalRate),
        results))

    for oreq in ors:
        fig, ax = plt.subplots(figsize=(6, 4), constrained_layout=True)

        plot_cdf(ax, psn.all_lats_exec(loc), get_color(psn))
        plot_cdf(ax, oreq.all_lats_exec(loc), ALTERNATE_ZIPF_COLOR, '--')

        format_cdf(ax)

        make_legend(ax, [legend_line(get_color(psn)),
            legend_line(ALTERNATE_ZIPF_COLOR, '--')][::-1], ['Poisson', 'Back-to-back'][::-1],
            ncol=1, loc='lower left', size=fontsize, squeeze=True)

        ax.set_xlabel('Latency (ms)', fontsize=fontsize)
        ax.set_ylabel('% Operations', fontsize=fontsize)
        # ax.set_xticks([0, 500, 1000, 1500, 2000])
        # ax.set_xticks([0, 1250, 2500, 3750, 5000])
        # ax.set_xticks([0, 500, 1000, 1500, 2000, 2500])
        ax.set_xticks([0, 500, 1000, 1500])
        ax.tick_params(axis='both', labelsize=fontsize)

        plt.savefig(path.join(dirname, 'or_vs_psn_cdf_{}_{}.pdf'.format(loc, oreq.arrival_rate())))

def infinite_bar_old(dirname):
    """
    Generates a bar graph that compares the execution latency of EPaxos with and
    without a modification that bounds execution delay. 'dirname' is a directory
    containing two EPaxos experiments with Zipfian workload, one with the fix
    and one without, as well as a Multi-Paxos experiment and EPaxos 0%
    experiment for comparison. The generated graph is saved as an image in the
    directory specified by 'dirname'.
    """
    plt.clf()

    results = get_results(dirname)
    inffix = list(filter(lambda r: is_epaxos_zipf_result(r) and r.inffix(),
        results))
    no_inffix = list(filter(lambda r: is_epaxos_zipf_result(r) \
        and not r.inffix(), results))
    epaxos_0 = get_fixed_epaxos_result(results, 0)
    mpaxos = get_mpaxos_result(results)
    expts = [epaxos_0, inffix, mpaxos]

    for e in no_inffix:
        for l in ORDERED_LOCS:
            print(l, max(e.all_lats_exec(l)))
        print()

    fig, ax = plt.subplots(1, 1, figsize=(8, 4), constrained_layout=True)
    barwidth = .25
    fontsize = 22

    # On the left graph, plot mean commit latency vs. mean exec latency vs.
    # mean exec latency without modification.
    def yfn(expt, loc):
        res = [
        # statistics.mean([e.mean_lat_commit(loc) for e in expt]),
        statistics.mean([e.mean_lat_exec(loc) for e in expt])]
        if expt == inffix:
            res.append(statistics.mean([x.mean_lat_exec(loc) for x in no_inffix]))
        return res
    def errfn(expt, loc):
        res = [
            # (min([e.mean_lat_commit(loc) for e in expt]), max([e.mean_lat_commit(loc) for e in expt])),
            (min([e.mean_lat_exec(loc) for e in expt]), max([e.mean_lat_exec(loc) for e in expt]))]
        if expt == inffix:
            res.append((min([x.mean_lat_exec(loc) for x in no_inffix]), max([x.mean_lat_exec(loc) for x in no_inffix])))
        return res
    plot_bar_by_loc(ax, expts, yfn, errfn, lambda e: get_color(e[0]), None, barwidth,
        fontsize, maxy=1250, ystep=250, hatches=[None, '//'], hatch_fill=[0],
        errwidth=7)

    hatches = [None, '//']

    ax.set_ylabel('Mean Latency (ms)', fontsize=fontsize)
    leg = make_legend(ax, [legend_patch_color(get_color(e[0])) for e in expts],
        [e[0].description() for e in expts], ncol=1, loc='upper left')
    make_legend(ax, [legend_patch_hatch(hatches[i]) for i in range(2)],
        ['Improved', 'Unmodified'], ncol=2, loc='upper right')
    ax.add_artist(leg)

    plt.savefig(path.join(dirname, 'infinite_bar_mean.pdf'))


    fig, ax = plt.subplots(1, 1, figsize=(8, 4), constrained_layout=True)

    # On the right graph, plot 99th percentile commit latency vs. 99th
    # percentile exec latency vs. 99th percentile exec latency without
    # modification.
    def yfn(expt, loc):
        res = [
        # statistics.mean([e.p99_lat_commit(loc) for e in expt]),
        statistics.mean([e.p99_lat_exec(loc) for e in expt])]
        if expt == inffix:
            res.append(statistics.mean([x.p99_lat_exec(loc) for x in no_inffix]))
        return res
    def errfn(expt, loc):
        res = [
            # (min([e.p99_lat_commit(loc) for e in expt]), max([e.p99_lat_commit(loc) for e in expt])),
            (min([e.p99_lat_exec(loc) for e in expt]), max([e.p99_lat_exec(loc) for e in expt]))]
        if expt == inffix:
            res.append((min([x.p99_lat_exec(loc) for x in no_inffix]), max([x.p99_lat_exec(loc) for x in no_inffix])))
        return res
    plot_bar_by_loc(ax, expts, yfn, errfn, lambda e: get_color(e[0]), None, barwidth,
        fontsize, maxy=5000, ystep=1000, hatches=[None, '//', '.'], hatch_fill=[0],
        errwidth=7)
    ax.set_ylabel('P99 Latency (ms)', fontsize=fontsize)

    plt.savefig(path.join(dirname, 'infinite_bar_p99.pdf'))

def infinite_bar(dirname):
    """
    Generates a bar graph that compares the execution latency of EPaxos with and
    without a modification that bounds execution delay. 'dirname' is a directory
    containing two EPaxos experiments with Zipfian workload, one with the fix
    and one without, as well as a Multi-Paxos experiment and EPaxos 0%
    experiment for comparison. The generated graph is saved as an image in the
    directory specified by 'dirname'.
    """
    plt.clf()

    results = get_results(dirname)
    inffix = list(filter(lambda r: is_epaxos_zipf_result(r) and r.inffix(),
        results))
    no_inffix = list(filter(lambda r: is_epaxos_zipf_result(r) \
        and not r.inffix(), results))
    epaxos_0 = get_fixed_epaxos_result(results, 0)
    mpaxos = get_mpaxos_result(results)
    expts = [no_inffix, inffix, mpaxos]

    # for e in no_inffix:
    #     for l in ORDERED_LOCS:
    #         print(l, max(e.all_lats_exec(l)))
    #     print()

    fig, ax = plt.subplots(1, 1, figsize=(8, 4), constrained_layout=True)
    barwidth = .25
    fontsize = 24

    color_fn = lambda e: get_color(e[0]) if e != no_inffix else ALTERNATE_ZIPF_COLOR

    # On the left graph, plot mean commit latency vs. mean exec latency vs.
    # mean exec latency without modification.
    def yfn(expt, loc):
        res = [
        # statistics.mean([e.mean_lat_commit(loc) for e in expt]),
        statistics.mean([e.mean_lat_exec(loc) for e in expt])]
        # if expt == inffix:
        #     res.append(statistics.mean([x.mean_lat_exec(loc) for x in no_inffix]))
        return res
    def errfn(expt, loc):
        res = [
            # (min([e.mean_lat_commit(loc) for e in expt]), max([e.mean_lat_commit(loc) for e in expt])),
            (min([e.mean_lat_exec(loc) for e in expt]), max([e.mean_lat_exec(loc) for e in expt]))]
        # if expt == inffix:
        #     res.append((min([x.mean_lat_exec(loc) for x in no_inffix]), max([x.mean_lat_exec(loc) for x in no_inffix])))
        return res
    plot_bar_by_loc(ax, expts, yfn, errfn, color_fn, None, barwidth,
        fontsize, maxy=1000, ystep=250, hatches=[None, '//'], hatch_fill=[0],
        errwidth=7)

    hatches = [None, '//']

    ax.set_ylabel('Mean Latency (ms)', fontsize=fontsize)
    leg = make_legend(ax, [legend_patch_color(color_fn(e)) for e in expts],
        ['Unmodified', 'Improved', 'MPaxos'], ncol=2, loc='upper left', size=22, squeeze=True)
    # make_legend(ax, [legend_patch_hatch(hatches[i]) for i in range(2)],
    #     ['Improved', 'Unmodified'], ncol=2, loc='upper right')
    # ax.add_artist(leg)

    plt.savefig(path.join(dirname, 'infinite_bar_mean.pdf'))


    fig, ax = plt.subplots(1, 1, figsize=(8, 4), constrained_layout=True)

    # On the right graph, plot 99th percentile commit latency vs. 99th
    # percentile exec latency vs. 99th percentile exec latency without
    # modification.
    def yfn(expt, loc):
        res = [
        # statistics.mean([e.p99_lat_commit(loc) for e in expt]),
        statistics.mean([e.p99_lat_exec(loc) for e in expt])]
        # if expt == inffix:
        #     res.append(statistics.mean([x.p99_lat_exec(loc) for x in no_inffix]))
        return res
    def errfn(expt, loc):
        res = [
            # (min([e.p99_lat_commit(loc) for e in expt]), max([e.p99_lat_commit(loc) for e in expt])),
            (min([e.p99_lat_exec(loc) for e in expt]), max([e.p99_lat_exec(loc) for e in expt]))]
        # if expt == inffix:
        #     res.append((min([x.p99_lat_exec(loc) for x in no_inffix]), max([x.p99_lat_exec(loc) for x in no_inffix])))
        return res
    plot_bar_by_loc(ax, expts, yfn, errfn, color_fn, None, barwidth,
        fontsize, maxy=4500, ystep=1000, hatches=[None, '//', '.'], hatch_fill=[0],
        errwidth=7)
    ax.set_ylabel('P99 Latency (ms)', fontsize=fontsize)

    plt.savefig(path.join(dirname, 'infinite_bar_p99.pdf'))

def client_metrics_over_time(dirname, loc='or'):
    plt.clf()

    results = get_results(dirname)
    expt = get_epaxos_zipf_result(results)[0]
    no_inffix = list(filter(lambda r: is_epaxos_zipf_result(r) and not r.inffix(),
        results))
    for i, expt in enumerate(results):#enumerate(no_inffix):
        fig, ax = plt.subplots(figsize=(5, 3), constrained_layout=True)

        # cap = 1500
        timestamps, lats, tputs, oreqs = expt.parse_lattput(loc)

        # ax.plot(timestamps, lats, label="Avg Latency")
        ax.plot(timestamps, tputs, label="Throughput")
        ax.plot(timestamps, oreqs, label="Outstanding")
        ax.plot(timestamps, [10*10 for _ in timestamps], label="Cap")

        ax.legend()

        ax.set_xlabel('Time (s)')
        ax.set_ylabel('# Operations')

        ax.grid()

        plt.savefig(path.join(dirname, 'client_metrics_over_time_{}_{}.pdf'.format(loc, expt.arrival_rate())))

if __name__ == '__main__':
    """
    Plots graphs for experiments that have already been run.
    """
    reproduction_bar('results/reproduction')
    batching_bar('results/batching')
    commitvexec_cdf('results/commitvexec')
    thrifty_bar('results/thrifty')
    osc_bar('results/osc')
    osc_bar_loc('results/osc')
    client_metrics_over_time('results/commitvexec')

