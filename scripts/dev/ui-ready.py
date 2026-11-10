#!/usr/bin/env python3
"""Observe rendered UI through AT-SPI, restricted to a child PID.

The supervisor runs this observer as a disposable process. Synchronous AT-SPI
calls can block; its external deadline/cancellation never depends on this loop.
"""
import sys
import time
import gi

gi.require_version('Atspi', '2.0')
from gi.repository import Atspi, GLib


def rendered(node, depth=0):
    if node is None or depth > 24:
        return False
    text = node.get_name() or ''
    if 'Text' in node.get_interfaces():
        text += ' ' + Atspi.Text.get_text(node, 0, -1)
    if any(value in text for value in ('Serviço local ativo', 'Serviço ativo', 'Aguardando serviço local')):
        return True
    return any(rendered(node.get_child_at_index(index), depth + 1)
               for index in range(node.get_child_count()))


def ready(pid):
    try:
        desktop = Atspi.get_desktop(0)
        for index in range(desktop.get_child_count()):
            app = desktop.get_child_at_index(index)
            if app.get_process_id() == pid and rendered(app):
                return True
    except GLib.Error:
        return False
    return False


if __name__ == '__main__':
    pid = int(sys.argv[1])
    while not ready(pid):
        time.sleep(.05)
    print('rendered pid=' + str(pid), flush=True)
