#!/usr/bin/env python3
"""
Parse perf logging file and generate a timeline report.
Collects all "PERF:" log lines, reverses order, and groups by digest.
"""

import re
import sys
from collections import defaultdict
from typing import List, Dict, Optional


def parse_log_line(line: str) -> Optional[dict]:
    """Parse a log line and extract PERF message if present."""
    # Match log format: time="..." level=info msg="PERF: ..."
    match = re.search(r'msg="PERF: (.+?)"', line)
    if not match:
        return None

    message = match.group(1)

    # Extract digest if present
    digest_match = re.search(r'digest=([^\s]+)', message)
    digest = digest_match.group(1) if digest_match else None

    size_match = re.search(r"size=(\d+)", message)
    size = size_match.group(1) if size_match else None

    time = total_from_log(message)

    return {
        'message': message,
        'digest': digest,
        'size': size,
        'time': time,
        'raw_line': line.strip()
    }


def parse_log_file(filepath: str) -> List[dict]:
    """Parse log file and return all PERF log entries."""
    perf_entries = []

    with open(filepath, 'r') as f:
        for line in f:
            entry = parse_log_line(line)
            if entry:
                perf_entries.append(entry)

    return perf_entries


def group_by_digest(entries: List[dict]) -> Dict[Optional[str], List[dict]]:
    """Group entries by digest."""
    grouped = defaultdict(list)
    for entry in entries:
        grouped[entry['digest']].append(entry)
    return grouped


def extract_overall_times(entries: List[dict]) -> tuple:
    """Extract pullCmd and copyLayers overall times."""
    cmd = None
    copy_layers = None

    for entry in entries:
        msg = entry['message']
        if 'pullCmd duration=' in msg or 'loadCmd duration=' in msg:
            cmd = entry
        elif 'copyLayers total=' in msg:
            copy_layers = entry

    return cmd, copy_layers


def total_from_log(message: str) -> float:
    match = re.search(r"total=(.+)", message)
    if match:
        time_str = match.group(1)
        if time_str.endswith('µs'):
            number_str = time_str[:-2]
            multiplier = 1e-6
        elif time_str.endswith('ns'):
            number_str = time_str[:-2]
            multiplier = 1e-9
        elif time_str.endswith('ms'):
            number_str = time_str[:-2]
            multiplier = 1e-3
        elif time_str.endswith('s'):
            number_str = time_str[:-1]
            multiplier = 1.0
        return float(number_str) * multiplier
    else:
        return 0.0


def main():
    if len(sys.argv) < 2:
        print("Usage: python parse_perf_log.py <log_file>", file=sys.stderr)
        sys.exit(1)

    log_file = sys.argv[1]

    # Parse all PERF entries
    entries = parse_log_file(log_file)

    # Reverse the order
    # this mostly fixes the display order except createNewLayer logs gets moved before
    # the putBlobToPendingFile stack which is wrong. I've been manually flipping that
    entries_reversed = list(reversed(entries))

    # Extract overall times
    cmd, copy_layers = extract_overall_times(entries_reversed)

    # Group by digest
    grouped = group_by_digest(entries_reversed)

    # Report overall times first
    print(cmd['message'])
    print(copy_layers['message'])

    # copied Layers
    layer_sizes = {}
    for entry in entries_reversed:
        if "copyLayer " in entry['message']:
            layer_sizes[entry['digest']] = entry['time']

    digest_order = dict(sorted(layer_sizes.items(), key=lambda item: item[1], reverse=True)).keys()

    # Sort digests by order of first appearance in reversed list
    # digest_order = []
    # seen_digests = set()
    # for entry in entries_reversed:
    #     if entry['digest'] and entry['digest'] not in seen_digests:
    #         digest_order.append(entry['digest'])
    #         seen_digests.add(entry['digest'])

    # only print first N digests
    n = 0
    n_digests = 3
    for digest in digest_order:
        if n > n_digests:
            break

        loglines = grouped[digest]

        # find the copy layer log line
        copy_layer_idx = -1
        for idx, entry in enumerate(loglines):
            if "copyLayer " in entry['message']:
                copy_layer_idx = idx

        # don't display if we are not copying a layer. it just adds noise
        if copy_layer_idx == -1:
            continue

        n += 1

        copy_layer_time = loglines[copy_layer_idx]['time']

        print(f"\nDigest: {digest}")
        for entry in loglines[copy_layer_idx:]:
            message = entry['message']
            percent = round(entry['time'] / copy_layer_time * 100, 2)
            message = message.replace(f" digest={digest}", "")
            message += f" %{percent}"
            print(message)


if __name__ == "__main__":
    main()

