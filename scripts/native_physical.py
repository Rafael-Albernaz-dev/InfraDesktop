"""Physical input primitives for owned X11 native test windows only."""


def focused(native, target, description):
    return native.wait_for(description, lambda: target.get_state_set().contains(native.Atspi.StateType.FOCUSED))


def key(native, ui, target, symbol, shift=False):
    # Observe focus before type_label_x11's grab_focus; never conceal a failed
    # transition by repairing its starting state inside the input helper.
    focused(native, target, 'physical key starting focus: ' + target.get_name())
    native.type_label_x11(ui, target, '', key_symbol=symbol, shift=shift)


def pointer(native, ui, app, target, button=1, count=1):
    assert ui.poll() is None and app.get_process_id() == ui.pid
    rect = native.Atspi.Component.get_extents(target, native.Atspi.CoordType.SCREEN)
    assert rect.width > 0 and rect.height > 0
    center = [rect.x + rect.width // 2, rect.y + rect.height // 2]
    x = native.C.CDLL('libX11.so.6')
    xt = native.C.CDLL('libXtst.so.6')
    x.XOpenDisplay.argtypes = [native.C.c_char_p]
    x.XOpenDisplay.restype = native.C.c_void_p
    x.XCloseDisplay.argtypes = [native.C.c_void_p]
    x.XSync.argtypes = [native.C.c_void_p, native.C.c_int]
    xt.XTestFakeMotionEvent.argtypes = [native.C.c_void_p, native.C.c_int, native.C.c_int, native.C.c_int, native.C.c_ulong]
    xt.XTestFakeButtonEvent.argtypes = [native.C.c_void_p, native.C.c_uint, native.C.c_int, native.C.c_ulong]
    display = x.XOpenDisplay(None)
    assert display
    try:
        assert xt.XTestFakeMotionEvent(display, -1, *center, 0)
        for _ in range(count):
            assert xt.XTestFakeButtonEvent(display, button, True, 0)
            assert xt.XTestFakeButtonEvent(display, button, False, 0)
        x.XSync(display, False)
    finally:
        x.XCloseDisplay(display)
    return center
