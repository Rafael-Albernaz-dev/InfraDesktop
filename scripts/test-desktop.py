#!/usr/bin/env python3
"""Native optimized Wails journey via AT-SPI, limited to owned subprocesses."""
import importlib.util
import ctypes as C
import json
import os
from pathlib import Path
import subprocess
import signal
import tempfile
import time
os.environ["GDK_BACKEND"]="x11"
import gi

gi.require_version('Atspi', '2.0')
from gi.repository import Atspi, GLib

spec = importlib.util.spec_from_file_location('lifecycle', Path(__file__).with_name('test-process.py'))
lifecycle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lifecycle)
DESKTOP_OUTPUT = lifecycle.ROOT / 'build/ui-021-desktop' / time.strftime('%Y%m%dT%H%M%S')


def wait_for(description, predicate, budget=10):
    until = time.monotonic() + budget
    while time.monotonic() < until:
        try:value = predicate()
        except GLib.Error:value=None # React may replace an AT-SPI node during this bounded condition; retry the same assertion.
        if value:
            return value
        time.sleep(0.1)
    raise AssertionError(description + ' was not observed within the native UI budget')


def owned_app(process):
    if process.poll() is not None:
        raise AssertionError('owned Wails exited before closing the window')
    desktop = Atspi.get_desktop(0)
    for i in range(desktop.get_child_count()):
        app = desktop.get_child_at_index(i)
        if app is not None and app.get_process_id() == process.pid:
            return app
    return None


def nodes(app):
    result = []
    def visit(a, depth=0):
        if depth > 20:
            return
        result.append(a)
        for i in range(a.get_child_count()):
            child = a.get_child_at_index(i)
            if child:
                visit(child, depth + 1)
    visit(app)
    return result


def snapshot_text(app):
    text = []
    for node in nodes(app):
        if node.get_name():
            text.append(node.get_name())
        if "Text" not in node.get_interfaces():
            continue
        try:
            value = Atspi.Text.get_text(node,0,-1)
            if value:
                text.append(value)
        except Exception:
            # Many GTK nodes have no Text interface; assertions use actual web text below.
            pass
    return '\n'.join(text)


def contents(app):
    try:
        return snapshot_text(app)
    except GLib.Error:
        # React replaces accessible nodes during snapshots; retry through wait_for,
        # retaining all assertions and the same bounded wait. Never substitute text.
        return ''


def find(app, role, name):
    for node in nodes(app):
        if node.get_role_name() == role and node.get_name() == name:
            return node
    return None


def x11_children(x,d,root):
    x.XQueryTree.argtypes=[C.c_void_p,C.c_ulong,C.POINTER(C.c_ulong),C.POINTER(C.c_ulong),C.POINTER(C.POINTER(C.c_ulong)),C.POINTER(C.c_uint)]
    rr=C.c_ulong();parent=C.c_ulong();children=C.POINTER(C.c_ulong)();count=C.c_uint()
    assert x.XQueryTree(d,root,C.byref(rr),C.byref(parent),C.byref(children),C.byref(count))
    result=list(children[:count.value]);x.XFree(children)
    x.XGetGeometry.argtypes=[C.c_void_p,C.c_ulong,C.POINTER(C.c_ulong),C.POINTER(C.c_int),C.POINTER(C.c_int),C.POINTER(C.c_uint),C.POINTER(C.c_uint),C.POINTER(C.c_uint),C.POINTER(C.c_uint)]
    visible=[]
    for window in result:
        rr=C.c_ulong();xx=C.c_int();yy=C.c_int();ww=C.c_uint();hh=C.c_uint();bb=C.c_uint();depth=C.c_uint()
        if x.XGetGeometry(d,window,C.byref(rr),C.byref(xx),C.byref(yy),C.byref(ww),C.byref(hh),C.byref(bb),C.byref(depth)) and ww.value>=500 and hh.value>=500:visible.append(window)
    return visible


def configure_window(process,width=1440,height=860):
    x=C.CDLL('libX11.so.6')
    x.XOpenDisplay.argtypes=[C.c_char_p];x.XOpenDisplay.restype=C.c_void_p
    x.XDefaultRootWindow.argtypes=[C.c_void_p];x.XDefaultRootWindow.restype=C.c_ulong
    x.XInternAtom.argtypes=[C.c_void_p,C.c_char_p,C.c_int];x.XInternAtom.restype=C.c_ulong
    x.XGetWindowProperty.argtypes=[C.c_void_p,C.c_ulong,C.c_ulong,C.c_long,C.c_long,C.c_int,C.c_ulong,C.POINTER(C.c_ulong),C.POINTER(C.c_int),C.POINTER(C.c_ulong),C.POINTER(C.c_ulong),C.POINTER(C.POINTER(C.c_ubyte))]
    x.XFree.argtypes=[C.c_void_p];x.XCloseDisplay.argtypes=[C.c_void_p]
    x.XSendEvent.argtypes=[C.c_void_p,C.c_ulong,C.c_int,C.c_long,C.c_void_p];x.XFlush.argtypes=[C.c_void_p]
    x.XMoveResizeWindow.argtypes=[C.c_void_p,C.c_ulong,C.c_int,C.c_int,C.c_uint,C.c_uint]
    x.XGetGeometry.argtypes=[C.c_void_p,C.c_ulong,C.POINTER(C.c_ulong),C.POINTER(C.c_int),C.POINTER(C.c_int),C.POINTER(C.c_uint),C.POINTER(C.c_uint),C.POINTER(C.c_uint),C.POINTER(C.c_uint)]
    d=x.XOpenDisplay(None);assert d
    def prop(w,n,kind):
        actual=C.c_ulong();fmt=C.c_int();count=C.c_ulong();after=C.c_ulong();data=C.POINTER(C.c_ubyte)()
        rc=x.XGetWindowProperty(d,w,x.XInternAtom(d,n.encode(),False),0,1024,False,kind,C.byref(actual),C.byref(fmt),C.byref(count),C.byref(after),C.byref(data))
        if rc!=0 or not data:return []
        values=list(C.cast(data,C.POINTER(C.c_ulong))[:count.value]) if fmt.value==32 else [];x.XFree(data);return values
    try:
        root=x.XDefaultRootWindow(d);owned=[w for w in (prop(root,'_NET_CLIENT_LIST',33) or x11_children(x,d,root)) if prop(w,'_NET_WM_PID',6)==[process.pid]];assert len(owned)==1;w=owned[0]
        class Data(C.Union):_fields_=[('b',C.c_char*20),('s',C.c_short*10),('l',C.c_long*5)]
        class Client(C.Structure):_fields_=[('type',C.c_int),('serial',C.c_ulong),('send_event',C.c_int),('display',C.c_void_p),('window',C.c_ulong),('message_type',C.c_ulong),('format',C.c_int),('data',Data)]
        class Event(C.Union):_fields_=[('client',Client),('pad',C.c_long*24)]
        ev=Event();ev.client.type=33;ev.client.send_event=1;ev.client.display=d;ev.client.window=w;ev.client.message_type=x.XInternAtom(d,b'_NET_WM_STATE',False);ev.client.format=32;ev.client.data.l[0]=0
        horizontal=x.XInternAtom(d,b'_NET_WM_STATE_MAXIMIZED_HORZ',False);vertical=x.XInternAtom(d,b'_NET_WM_STATE_MAXIMIZED_VERT',False)
        ev.client.data.l[1]=horizontal;ev.client.data.l[2]=vertical;ev.client.data.l[3]=1
        assert x.XSendEvent(d,root,False,(1<<20)|(1<<19),C.byref(ev));x.XFlush(d)
        wait_for('owned unmaximized window',lambda:not ({horizontal,vertical}&set(prop(w,'_NET_WM_STATE',4))))
        x.XMoveResizeWindow(d,w,40,30,width,height);x.XFlush(d)
        def sized():
            rr=C.c_ulong();xx=C.c_int();yy=C.c_int();ww=C.c_uint();hh=C.c_uint();bb=C.c_uint();depth=C.c_uint()
            assert x.XGetGeometry(d,w,C.byref(rr),C.byref(xx),C.byref(yy),C.byref(ww),C.byref(hh),C.byref(bb),C.byref(depth));return ww.value==width and hh.value==height
        try:wait_for('owned content geometry '+str(width)+'x'+str(height),sized)
        except AssertionError:
            print('window sizing diagnostic:',prop(w,'_NET_WM_STATE',4), 'frame_extents',prop(w,'_NET_FRAME_EXTENTS',6))
            rr=C.c_ulong();xx=C.c_int();yy=C.c_int();ww=C.c_uint();hh=C.c_uint();bb=C.c_uint();depth=C.c_uint();x.XGetGeometry(d,w,C.byref(rr),C.byref(xx),C.byref(yy),C.byref(ww),C.byref(hh),C.byref(bb),C.byref(depth));print('actual owned X11 client geometry',ww.value,hh.value)
            raise
        icon=prop(w,'_NET_WM_ICON',6)
        if icon:
            assert icon[0]>0 and icon[1]>0,'invalid EWMH icon'
        else:
            # GTK uses ICCCM pixmap/mask when the X server has no EWMH window manager.
            class Hints(C.Structure):_fields_=[('flags',C.c_long),('input',C.c_int),('initial_state',C.c_int),('icon_pixmap',C.c_ulong),('icon_window',C.c_ulong),('icon_x',C.c_int),('icon_y',C.c_int),('icon_mask',C.c_ulong),('window_group',C.c_ulong)]
            x.XGetWMHints.argtypes=[C.c_void_p,C.c_ulong];x.XGetWMHints.restype=C.POINTER(Hints)
            hints=x.XGetWMHints(d,w);assert hints,'missing actual window icon hints';h=hints.contents
            assert h.flags&4 and h.flags&32 and h.icon_pixmap and h.icon_mask,'missing actual icon pixmap/mask'
            rr=C.c_ulong();xx=C.c_int();yy=C.c_int();ww=C.c_uint();hh=C.c_uint();bb=C.c_uint();depth=C.c_uint()
            assert x.XGetGeometry(d,h.icon_pixmap,C.byref(rr),C.byref(xx),C.byref(yy),C.byref(ww),C.byref(hh),C.byref(bb),C.byref(depth))
            x.XGetImage.argtypes=[C.c_void_p,C.c_ulong,C.c_int,C.c_int,C.c_uint,C.c_uint,C.c_ulong,C.c_int];x.XGetImage.restype=C.c_void_p
            x.XGetPixel.argtypes=[C.c_void_p,C.c_int,C.c_int];x.XGetPixel.restype=C.c_ulong;x.XDestroyImage.argtypes=[C.c_void_p]
            image=x.XGetImage(d,h.icon_pixmap,0,0,ww.value,hh.value,0xffffffff,2);mask=x.XGetImage(d,h.icon_mask,0,0,ww.value,hh.value,0xffffffff,2);assert image and mask
            from PIL import Image
            pixels=[]
            for yy in range(hh.value):
                for xx in range(ww.value):
                    color=x.XGetPixel(image,xx,yy);alpha=255 if x.XGetPixel(mask,xx,yy) else 0;pixels.append(((color>>16)&255,(color>>8)&255,color&255,alpha))
            x.XDestroyImage(image);x.XDestroyImage(mask);x.XFree(hints)
            assert (14,116,144,255) in pixels and (255,255,255,255) in pixels,'icon colors differ from integrated asset'
            pix=Image.new('RGBA',(ww.value,hh.value));pix.putdata(pixels);target=DESKTOP_OUTPUT / 'window-icon.png';target.parent.mkdir(parents=True,exist_ok=True);pix.save(target)
    finally:x.XCloseDisplay(d)


def open_ui(runtime,client,expected="Serviço ativo",owned_home=None,viewport=(1440,860)):
    env = dict(os.environ, INFRADESKTOP_RUNTIME=str(runtime), INFRADESKTOP_CLIENT_STATE=str(client), NO_AT_BRIDGE='0', GTK_MODULES='atk-bridge', GDK_BACKEND='x11')
    test_cwd = None
    if owned_home is not None:
        test_cwd = Path(owned_home)
        assert test_cwd.parent == Path(client).parent
        test_cwd.mkdir(mode=0o700, exist_ok=True)
        # A synthetic child home isolates GTK bookmarks/recent files and the
        # native picker from the human profile. No host environment is changed.
        env['HOME'] = str(test_cwd)
        env['GTK_USE_PORTAL'] = '0'
        env['GIO_USE_VFS'] = 'local'
        env['GIO_USE_VOLUME_MONITOR'] = 'unix'
        env['GSETTINGS_BACKEND'] = 'memory'
        gio_modules = test_cwd / 'gio-modules'
        gio_modules.mkdir(mode=0o700, exist_ok=True)
        assert not any(gio_modules.iterdir())
        env['GIO_MODULE_DIR'] = str(gio_modules)
        env.pop('GIO_EXTRA_MODULES', None)
        for key, suffix in [('XDG_CONFIG_HOME','config'),('XDG_DATA_HOME','data'),('XDG_CACHE_HOME','cache')]:
            location = test_cwd / suffix
            location.mkdir(mode=0o700, exist_ok=True)
            env[key] = str(location)
    print('owned UI command: INFRADESKTOP_RUNTIME='+str(runtime)+' GDK_BACKEND=x11 '+str(lifecycle.ROOT / 'build/bin/infradesktop'),flush=True)
    started=time.monotonic()
    process = subprocess.Popen([str(lifecycle.ROOT / 'build/bin/infradesktop')], env=env, cwd=test_cwd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    try:
        app = wait_for('owned Wails accessibility application', lambda: owned_app(process))
        wait_for('connected Wails snapshot', lambda: expected in contents(app))
        openings.append(time.monotonic()-started)
        configure_window(process,*viewport)
        print('owned optimized Wails pid=' + str(process.pid) + ' connected (real GTK/WebKit, X11 backend)')
        return process, app
    except BaseException:
        if process.poll() is None:
            app=owned_app(process)
            if app:print('owned startup failed; secret-safe diagnosis: PID',process.pid)
        terminate_ui(process)
        raise


def type_label_x11(process, entry, text, activate=False, keyboard_focus=False, key_symbol=None, shift=False):
    # AT-SPI keyboard synthesis does not reach this session's WebKit input.
    # XTest sends physical key events after focusing a window validated by its owned PID.
    x=C.CDLL('libX11.so.6');xt=C.CDLL('libXtst.so.6')
    x.XOpenDisplay.argtypes=[C.c_char_p];x.XOpenDisplay.restype=C.c_void_p
    x.XDefaultRootWindow.argtypes=[C.c_void_p];x.XDefaultRootWindow.restype=C.c_ulong
    x.XInternAtom.argtypes=[C.c_void_p,C.c_char_p,C.c_int];x.XInternAtom.restype=C.c_ulong
    x.XGetWindowProperty.argtypes=[C.c_void_p,C.c_ulong,C.c_ulong,C.c_long,C.c_long,C.c_int,C.c_ulong,C.POINTER(C.c_ulong),C.POINTER(C.c_int),C.POINTER(C.c_ulong),C.POINTER(C.c_ulong),C.POINTER(C.POINTER(C.c_ubyte))]
    x.XFree.argtypes=[C.c_void_p];x.XCloseDisplay.argtypes=[C.c_void_p]
    x.XSetInputFocus.argtypes=[C.c_void_p,C.c_ulong,C.c_int,C.c_ulong]
    x.XKeysymToKeycode.argtypes=[C.c_void_p,C.c_ulong];x.XKeysymToKeycode.restype=C.c_uint
    x.XSync.argtypes=[C.c_void_p,C.c_int]
    xt.XTestFakeKeyEvent.argtypes=[C.c_void_p,C.c_uint,C.c_int,C.c_ulong]
    display=x.XOpenDisplay(None);assert display
    def prop(window,name,kind):
        actual=C.c_ulong();fmt=C.c_int();count=C.c_ulong();after=C.c_ulong();data=C.POINTER(C.c_ubyte)()
        rc=x.XGetWindowProperty(display,window,x.XInternAtom(display,name.encode(),False),0,1024,False,kind,C.byref(actual),C.byref(fmt),C.byref(count),C.byref(after),C.byref(data))
        if rc!=0 or not data:return []
        values=list(C.cast(data,C.POINTER(C.c_ulong))[:count.value]) if fmt.value==32 else []
        x.XFree(data);return values
    try:
        owned=[w for w in (prop(x.XDefaultRootWindow(display),'_NET_CLIENT_LIST',33) or x11_children(x,display,x.XDefaultRootWindow(display))) if prop(w,'_NET_WM_PID',6)==[process.pid]]
        assert len(owned)==1
        assert Atspi.Component.grab_focus(entry)
        x.XSetInputFocus(display,owned[0],2,0)
        def key(sym,down):
            code=x.XKeysymToKeycode(display,sym);assert code
            assert xt.XTestFakeKeyEvent(display,code,down,0)
        if key_symbol is not None:
            if shift:key(0xffe1,True)
            key(key_symbol,True);key(key_symbol,False)
            if shift:key(0xffe1,False)
            x.XSync(display,False);return
        if activate:
            if keyboard_focus:
                # Real Tab navigation establishes :focus-visible in WebKit;
                # AT-SPI grab_focus alone establishes focus without modality.
                key(0xff09, True);key(0xff09, False)
                key(0xffe1, True);key(0xff09, True);key(0xff09, False);key(0xffe1, False)
            key(0xff0d,True);key(0xff0d,False);x.XSync(display,False);return
        key(0xffe3,True);key(ord('a'),True);key(ord('a'),False);key(0xffe3,False)
        for char in text:
            if char.isupper():key(0xffe1,True)
            key(ord(char.lower()),True);key(ord(char.lower()),False)
            if char.isupper():key(0xffe1,False)
        x.XSync(display,False)
    finally:x.XCloseDisplay(display)


def request_close_window(process, app):
    button = find(app, 'push button', 'Close')
    if button:
        assert button.get_action_iface().do_action(0)
    else:
        # Under X11, window-manager decorations are outside the GTK accessibility tree.
        # Send the same WM close protocol to a window whose _NET_WM_PID is our child PID.
        x = C.CDLL('libX11.so.6')
        x.XOpenDisplay.argtypes=[C.c_char_p];x.XOpenDisplay.restype=C.c_void_p
        x.XDefaultRootWindow.argtypes=[C.c_void_p];x.XDefaultRootWindow.restype=C.c_ulong
        x.XInternAtom.argtypes=[C.c_void_p,C.c_char_p,C.c_int];x.XInternAtom.restype=C.c_ulong
        x.XGetWindowProperty.argtypes=[C.c_void_p,C.c_ulong,C.c_ulong,C.c_long,C.c_long,C.c_int,C.c_ulong,C.POINTER(C.c_ulong),C.POINTER(C.c_int),C.POINTER(C.c_ulong),C.POINTER(C.c_ulong),C.POINTER(C.POINTER(C.c_ubyte))]
        x.XFree.argtypes=[C.c_void_p]
        x.XSendEvent.argtypes=[C.c_void_p,C.c_ulong,C.c_int,C.c_long,C.c_void_p]
        x.XFlush.argtypes=[C.c_void_p];x.XCloseDisplay.argtypes=[C.c_void_p]
        display=x.XOpenDisplay(None);assert display
        def prop(window,name,kind):
            actual=C.c_ulong();fmt=C.c_int();count=C.c_ulong();after=C.c_ulong();data=C.POINTER(C.c_ubyte)()
            atom=x.XInternAtom(display,name.encode(),False)
            rc=x.XGetWindowProperty(display,window,atom,0,1024,False,kind,C.byref(actual),C.byref(fmt),C.byref(count),C.byref(after),C.byref(data))
            if rc!=0 or not data:return []
            values=list(C.cast(data,C.POINTER(C.c_ulong))[:count.value]) if fmt.value==32 else []
            x.XFree(data);return values
        try:
            windows=(prop(x.XDefaultRootWindow(display),'_NET_CLIENT_LIST',33) or x11_children(x,display,x.XDefaultRootWindow(display)))
            owned=[w for w in windows if prop(w,'_NET_WM_PID',6)==[process.pid]]
            assert len(owned)==1,'expected one X11 window owned by the Wails child'
            class Data(C.Union):_fields_=[('b',C.c_char*20),('s',C.c_short*10),('l',C.c_long*5)]
            class Client(C.Structure):_fields_=[('type',C.c_int),('serial',C.c_ulong),('send_event',C.c_int),('display',C.c_void_p),('window',C.c_ulong),('message_type',C.c_ulong),('format',C.c_int),('data',Data)]
            class Event(C.Union):_fields_=[('client',Client),('pad',C.c_long*24)]
            event=Event();event.client.type=33;event.client.send_event=1;event.client.display=display;event.client.window=owned[0];event.client.message_type=x.XInternAtom(display,b'WM_PROTOCOLS',False);event.client.format=32;event.client.data.l[0]=x.XInternAtom(display,b'WM_DELETE_WINDOW',False)
            assert x.XSendEvent(display,owned[0],False,0,C.byref(event))
            x.XFlush(display)
        finally:x.XCloseDisplay(display)


def close_window(process, app):
    request_close_window(process, app)
    process.wait(timeout=5)
    assert process.returncode == 0
    print('native window Close protocol: owned Wails pid=' + str(process.pid) + ' exit=0')


def terminate_ui(process):
    if process and process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
    if process:
        print('owned Wails cleanup pid='+str(process.pid)+' exit='+str(process.returncode))
        error=process.stderr.read()
        if error: print('owned Wails stderr:',error[:2000])


def click(app,name):
    button=wait_for('button '+name,lambda:find(app,'push button',name) or find(app,'toggle button',name))
    assert button.get_action_iface().do_action(0),'native button action failed'


def type_field(process,app,name,value):
    entry=wait_for('field '+name,lambda:find(app,'entry',name) or find(app,'password text',name))
    type_label_x11(process,entry,value)
    wait_for("field input length "+name,lambda:Atspi.Text.get_character_count(entry)==len(value))


def capture(process,target,preserve_focus=False):
    # Capture only the owned X11 client surface, never the user's desktop.
    gi.require_version('Gdk','3.0');gi.require_version('GdkX11','3.0')
    from gi.repository import Gdk,GdkX11
    Gdk.set_allowed_backends("x11")
    display=Gdk.Display.open(os.environ['DISPLAY']);assert display
    # A private connection avoids cached foreign windows when X11 reuses an XID.
    x=C.CDLL('libX11.so.6')
    x.XOpenDisplay.argtypes=[C.c_char_p];x.XOpenDisplay.restype=C.c_void_p
    x.XDefaultRootWindow.argtypes=[C.c_void_p];x.XDefaultRootWindow.restype=C.c_ulong
    x.XInternAtom.argtypes=[C.c_void_p,C.c_char_p,C.c_int];x.XInternAtom.restype=C.c_ulong
    x.XGetWindowProperty.argtypes=[C.c_void_p,C.c_ulong,C.c_ulong,C.c_long,C.c_long,C.c_int,C.c_ulong,C.POINTER(C.c_ulong),C.POINTER(C.c_int),C.POINTER(C.c_ulong),C.POINTER(C.c_ulong),C.POINTER(C.POINTER(C.c_ubyte))]
    x.XFree.argtypes=[C.c_void_p];x.XCloseDisplay.argtypes=[C.c_void_p]
    d=x.XOpenDisplay(None);assert d
    def prop(w,n,kind):
        actual=C.c_ulong();fmt=C.c_int();count=C.c_ulong();after=C.c_ulong();data=C.POINTER(C.c_ubyte)()
        rc=x.XGetWindowProperty(d,w,x.XInternAtom(d,n.encode(),False),0,1024,False,kind,C.byref(actual),C.byref(fmt),C.byref(count),C.byref(after),C.byref(data))
        if rc!=0 or not data:return []
        values=list(C.cast(data,C.POINTER(C.c_ulong))[:count.value]) if fmt.value==32 else [];x.XFree(data);return values
    try:
        owned=[w for w in (prop(x.XDefaultRootWindow(d),'_NET_CLIENT_LIST',33) or x11_children(x,d,x.XDefaultRootWindow(d))) if prop(w,'_NET_WM_PID',6)==[process.pid]];assert len(owned)==1
        x.XWarpPointer.argtypes=[C.c_void_p,C.c_ulong,C.c_ulong,C.c_int,C.c_int,C.c_uint,C.c_uint,C.c_int,C.c_int]
        x.XSync.argtypes=[C.c_void_p,C.c_int]
        xt=C.CDLL('libXtst.so.6');xt.XTestFakeButtonEvent.argtypes=[C.c_void_p,C.c_uint,C.c_int,C.c_ulong]
        if not preserve_focus:
            login_capture='Entrar' in contents(owned_app(process))
            neutral=(72,66) if login_capture else (24,28)
            x.XWarpPointer(d,0,owned[0],0,0,0,0,*neutral);assert xt.XTestFakeButtonEvent(d,1,True,0);assert xt.XTestFakeButtonEvent(d,1,False,0);x.XSync(d,False)
            wait_for('neutral capture focus',lambda:not any(n.get_role_name() in ('push button','toggle button','entry','password text','check box') and n.get_state_set().contains(Atspi.StateType.FOCUSED) for n in nodes(owned_app(process))))
        w=GdkX11.X11Window.foreign_new_for_display(display,owned[0])
        # get_geometry queries X11; get_width/get_height depend on processed Configure events.
        measured=w.get_geometry();pix=Gdk.pixbuf_get_from_window(w,0,0,measured.width,measured.height);assert pix
        assert w.get_geometry()==measured,'Window changed during native capture'
        target.parent.mkdir(parents=True,exist_ok=True);pix.savev(str(target),'png',[],[])
        return {'file':str(target.relative_to(lifecycle.ROOT)),'size':[pix.get_width(),pix.get_height()],'pid':process.pid}
    finally:
        x.XCloseDisplay(d)
        display.close()



openings=[]
def main():
    import re,sqlite3
    spec=importlib.util.spec_from_file_location('resources',Path(__file__).with_name('measure-resources.py'));resources=importlib.util.module_from_spec(spec);spec.loader.exec_module(resources)
    if not os.environ.get('DISPLAY'):raise SystemExit('BLOCKED: private X11 display required')
    report=[];out=DESKTOP_OUTPUT;out.mkdir(parents=True,exist_ok=True)
    measurements={'hardware':resources.hardware(),'binaries':resources.binary_hashes(lifecycle.ROOT),'memory':[],'idle':[],'navigation':[],'openings_seconds':openings}
    password='isolated desktop password';new_password='new isolated desktop password'
    with tempfile.TemporaryDirectory(prefix='id-ui-021-desktop-') as tmp:
        root=Path(tmp);runtime=root/'runtime';state=root/'state';client=root/'client';runtime.mkdir(mode=0o700);client.mkdir(mode=0o700)
        daemon=lifecycle.start(runtime,state);ui=None
        def wait_text(text,budget=10):return wait_for(text,lambda:text in contents(app),budget)
        def theme(next,back):
            click(app,'Abrir configurações');click(app,'Escuro' if next=='dark' else 'Claro')
            def saved_theme():
                with sqlite3.connect(state/'infradesktop.db') as db:return db.execute('select theme from account').fetchone()[0]==next
            wait_for('persisted theme '+next,saved_theme)
            if back==2:click(app,'Selecionar empresa');wait_text('Empresas')
            elif back==3:click(app,'Dashboard');wait_text('Inventário disponível nesta empresa')
        def save(name):report.append(capture(ui,out/name))
        def access_theme(next, name):
            from PIL import Image
            label = 'Escuro' if next == 'dark' else 'Claro'
            button = wait_for('access theme ' + label, lambda: find(app, 'toggle button', label) or find(app, 'push button', label))
            # Focus and activate through the real X11 keyboard path.
            type_label_x11(ui, button, '', activate=True, keyboard_focus=True)
            expected = (27, 28, 32) if next == 'dark' else (255, 255, 255)
            target = out / name
            def painted():
                capture(ui, target, preserve_focus=True)
                return Image.open(target).convert('RGB').getpixel((8, 80)) == expected
            wait_for('painted access theme ' + next, painted)
            report.append({'step': 'preauth keyboard theme ' + next, 'file': str(target.relative_to(lifecycle.ROOT)), 'PASS': True})
        def recovery_actions(flow,code):
            gi.require_version('Gtk','3.0');gi.require_version('Gdk','3.0')
            from gi.repository import Gtk,Gdk
            click(app,'Copiar código');wait_text('Código copiado.')
            clipboard=Gtk.Clipboard.get(Gdk.SELECTION_CLIPBOARD)
            assert clipboard.wait_for_text()==code.replace('-',''),'native copy differs from issuance'
            clipboard.set_text('',-1)
            def chooser():
                return next((n for n in nodes(app) if n.get_role_name() in ('file chooser','dialog') and n.get_name()=='Salvar código de recuperação'),None)
            def chooser_button(panel,names):
                return next((n for n in nodes(panel) if n.get_role_name()=='push button' and n.get_name() in names),None)
            click(app,'Salvar .txt');panel=wait_for('owned native save dialog',chooser)
            cancel=wait_for('native cancel',lambda:chooser_button(panel,('Cancel','Cancelar')))
            assert cancel.get_action_iface().do_action(0);wait_for('native save cancellation',lambda:chooser() is None)
            assert recovery_code()==code,'cancel changed code'
            click(app,'Salvar .txt');panel=wait_for('owned native save dialog',chooser)
            field=wait_for('native filename',lambda:next((n for n in nodes(panel) if n.get_role_name()=='text' and 'EditableText' in n.get_interfaces() or n.get_role_name()=='entry' and 'EditableText' in n.get_interfaces()),None))
            target=root/(flow+'-recovery.txt')
            assert Atspi.EditableText.set_text_contents(field,str(target))
            button=wait_for('native save button',lambda:chooser_button(panel,('Save','Salvar')))
            assert button.get_action_iface().do_action(0)
            wait_for('native saved file',lambda:target.exists());wait_text('Arquivo salvo.')
            assert target.read_text()==code.replace('-','')+'\n\nCódigo de recuperação InfraDesktop. Guarde offline em local seguro.\n'
            assert target.stat().st_mode&0o777==0o600
            assert recovery_code()==code,'save acknowledged code'
            target.unlink()
            label='Novo código de recuperação' if flow=='renew' else 'Código de recuperação'
            field=wait_for('pending code keyboard target',lambda:next((n for n in nodes(app) if n.get_name()==label and 'Text'in n.get_interfaces()),None))
            type_label_x11(ui,field,'',key_symbol=0xff09)
            wait_for('native Tab reaches Copy from textarea',lambda:find(app,'push button','Copiar código').get_state_set().contains(Atspi.StateType.FOCUSED))
            type_label_x11(ui,find(app,'push button','Copiar código'),'',key_symbol=0xff09,shift=True)
            wait_for('native ShiftTab returns to textarea',lambda:field.get_state_set().contains(Atspi.StateType.FOCUSED))
            if flow=='renew':
                type_label_x11(ui,find(app,'push button','Guardei meu código'),'',key_symbol=0xff09)
                wait_for('native dialog Tab trap',lambda:find(app,'push button','Fechar painel').get_state_set().contains(Atspi.StateType.FOCUSED))
                type_label_x11(ui,field,'',key_symbol=0xff1b)
                assert find(app,'dialog','Segurança') and recovery_code()==code,'Escape discarded pending code'
            report.append({'step':flow+' real Tab/ShiftTab textarea and pending focus','PASS':True})
            report.append({'step':flow+' native clipboard/save dialog/cancel/exact bytes/private mode/no ack','PASS':True})
        def one_time_mutation(submit):
            suspended_at=time.monotonic()
            os.kill(daemon.pid, signal.SIGSTOP)  # SIGSTOP only this isolated daemon.
            try:
                click(app, submit)
                back = wait_for('locked one-time navigation', lambda: find(app, 'push button', 'Voltar ao login'))
                wait_for('one-time busy lock', lambda: not back.get_state_set().contains(Atspi.StateType.SENSITIVE))
                request_close_window(ui, app)
                assert ui.poll() is None, 'native close discarded in-flight access emission'
                # Native activation of disabled navigation cannot abandon the result.
                back.get_action_iface().do_action(0)
            finally:
                os.kill(daemon.pid, signal.SIGCONT)  # Only this owned isolated daemon.
                elapsed=time.monotonic()-suspended_at
                report.append({'step':'bounded pause during '+submit,'seconds':elapsed})
                print('owned daemon mutation pause seconds='+str(round(elapsed,3)),flush=True)
                assert elapsed<3,'Native automation exceeded the real IPC deadline; run without concurrent rendering load'

        try:
            ui,app=open_ui(runtime,client)
            access_theme('dark', 'M01-access-dark-focused.png')
            access_theme('light', 'M01-access-light-focused.png')
            save('M01-light.png')
            with sqlite3.connect(state/'infradesktop.db') as db:assert db.execute('select count(*) from account').fetchone()[0]==0
            click(app,'Criar conta');wait_text('Crie sua conta local')
            type_field(ui,app,'Usuário','desktop-technical');type_field(ui,app,'Senha',password);one_time_mutation('Criar conta local')
            def recovery_code():
                match=re.search(r'(?:[a-f0-9]{8}-){7}[a-f0-9]{8}',contents(app));return match.group(0) if match else None
            recovery=wait_for('one-time offline code',recovery_code)
            request_close_window(ui,app)
            click(app,'Escuro');assert recovery_code() == recovery
            click(app,'Claro');assert recovery_code() == recovery
            assert ui.poll() is None,'native close discarded registration code'
            report.append({'step':'pending register/native close/navigation/code survives themes until ack','PASS':True})
            recovery_actions('register',recovery)
            lifecycle.stop(daemon);wait_text('Reconectando ao serviço');daemon=lifecycle.start(runtime,state);wait_text('Serviço ativo')
            click(app,'Copiar código');wait_text('Este código não está mais disponível.')
            request_close_window(ui,app);assert ui.poll() is None
            report.append({'step':'preauth daemon restart refuses stale receipt; explicit discard remains possible','PASS':True})
            click(app,'Guardei meu código');wait_text('Entrar')
            # Window-local dark must not overwrite the account's persisted light.
            click(app,'Escuro')
            type_field(ui,app,'Senha',password);click(app,'Entrar');wait_text('Empresas')
            with sqlite3.connect(state/'infradesktop.db') as db:assert db.execute('select theme from account').fetchone()[0]=='light'
            report.append({'step':'account theme wins after local dark access theme','PASS':True})
            for name in ['Native Alpha','Native Beta']:
                click(app,'Nova empresa');type_field(ui,app,'Nome da empresa',name);click(app,'Criar empresa')
                wait_for('real company '+name,lambda:name in contents(app) and not find(app,'entry','Nome da empresa'))
            save('M02-light.png');theme('dark',2);save('M02-dark.png')
            alpha=wait_for('Native Alpha selectable row',lambda:next((n for n in nodes(app) if n.get_name()=='Selecionar Native Alpha'),None))
            type_label_x11(ui,alpha,'',activate=True)
            wait_for('selected Native Alpha detail',lambda:find(app,'heading','Native Alpha'))
            click(app,'Abrir empresa');wait_text('Inventário disponível nesta empresa');click(app,'Adicionar equipamento');wait_for('real equipment form',lambda:find(app,'entry','IP de gerenciamento'));click(app,'Fechar painel');save('M03-dark.png')
            theme('light',3);save('M03-light.png')
            for name,num in [('Equipamentos',4),('Topologia',6),('Abrir configurações',7)]:
                click(app,name);wait_text('Aparência' if num==7 else 'Inventário disponível nesta empresa');save('M0'+str(num)+'-light.png')
            # Draft cannot change saved company or claim a successful save.
            type_field(ui,app,'Nome visual','Unpersisted Draft');wait_text('Unpersisted Draft');click(app,'Cancelar')
            save('BF04-after-cancel.png')
            wait_for('cancel restores empty visual name',lambda:Atspi.Text.get_text(find(app,'entry','Nome visual'),0,-1)=='')
            assert '412' not in contents(app)
            type_field(ui,app,'Nome visual','Visual Alpha');click(app,'Rosa');click(app,'Salvar')
            def saved_brand():
                with sqlite3.connect(state/'infradesktop.db') as db:
                    profile=db.execute('SELECT profile FROM account').fetchone()[0]
                    assert db.execute('SELECT name FROM companies WHERE id=?',(profile,)).fetchone()[0]=='Native Alpha'
                with sqlite3.connect(state/'companies'/(profile+'.db')) as db:return db.execute('SELECT name,accent FROM branding').fetchone()==('Visual Alpha','rose')
            wait_for('brand saved in exact company unit',saved_brand);wait_text('Visual Alpha')
            report.append({'step':'native appearance save persists visual name/accent in Alpha unit, catalog unchanged','PASS':True})
            click(app,'Escuro');save('M07-dark.png');click(app,'Claro');click(app,'Dashboard');wait_text('Inventário disponível nesta empresa')
            # BF03 new Security surface: capture only with blank password/no code.
            click(app,'Conta: desktop-technical');click(app,'Segurança');wait_text('O código de recuperação pertence')
            def security_theme(next):
                click(app,'Escuro' if next=='dark' else 'Claro')
                def saved():
                    with sqlite3.connect(state/'infradesktop.db') as db:return db.execute('select theme from account').fetchone()[0]==next
                wait_for('persisted Security theme',saved)
                button=wait_for('Security theme state',lambda:find(app,'toggle button','Escuro' if next=='dark' else 'Claro'))
                wait_for('Security selected theme',lambda:button.get_state_set().contains(Atspi.StateType.PRESSED))
            from PIL import Image
            for security_mode in ['light','dark']:
                security_theme(security_mode)
                target=out/('BF03-security-'+security_mode+'.png')
                def painted():
                    capture(ui,target,preserve_focus=True)
                    dialog=wait_for('Security dialog geometry',lambda:find(app,'dialog','Segurança'))
                    rect=Atspi.Component.get_extents(dialog,Atspi.CoordType.WINDOW)
                    return Image.open(target).convert('RGB').getpixel((rect.x+12,rect.y+12))==((27,28,32) if security_mode=='dark' else (255,255,255))
                wait_for('painted Security theme '+security_mode,painted)
                report.append({'file':str(target.relative_to(lifecycle.ROOT)),'step':'Security '+security_mode+' blank password/no code','PASS':True})
            # H03: force a real IPC timeout using only the owned daemon. No
            # stubbed bindings or secret-bearing screenshots. The first result
            # is unknown; reopening must not silently turn retry into issuance.
            security_theme('light');type_field(ui,app,'Senha atual',password)
            os.kill(daemon.pid,signal.SIGSTOP)
            try:
                click(app,'Gerar novo código')
                submit=wait_for('renewal busy button',lambda:find(app,'push button','Gerar novo código'))
                wait_for('renewal in-flight',lambda:not submit.get_state_set().contains(Atspi.StateType.SENSITIVE))
                request_close_window(ui,app)
                wait_text('Não recebemos a confirmação')
                assert ui.poll() is None,'native close discarded in-flight renewal'
            finally:
                os.kill(daemon.pid,signal.SIGCONT)
            wait_text('Serviço ativo')
            click(app,'Fechar painel');wait_for('uncertain panel closed',lambda:not find(app,'dialog','Segurança'))
            click(app,'Conta: desktop-technical');click(app,'Segurança');wait_text('Não recebemos a confirmação')
            assert not recovery_code(),'reopening silently emitted a recovery code'
            for security_mode in ['light','dark']:
                security_theme(security_mode)
                field=wait_for('empty uncertainty password',lambda:find(app,'password text','Senha atual') or find(app,'entry','Senha atual'))
                assert Atspi.Text.get_text(field,0,-1)=='','uncertain password was retained'
                target=out/('BF03-security-uncertain-'+security_mode+'.png')
                def painted_uncertainty():
                    capture(ui,target,preserve_focus=True)
                    dialog=wait_for('Security dialog geometry',lambda:find(app,'dialog','Segurança'))
                    rect=Atspi.Component.get_extents(dialog,Atspi.CoordType.WINDOW)
                    return Image.open(target).convert('RGB').getpixel((rect.x+12,rect.y+12))==((27,28,32) if security_mode=='dark' else (255,255,255))
                wait_for('painted Security uncertainty '+security_mode,painted_uncertainty)
                report.append({'file':str(target.relative_to(lifecycle.ROOT)),'step':'H03 uncertainty survives close/reopen '+security_mode+'; empty password/no code','PASS':True})
            security_theme('light');type_field(ui,app,'Senha atual',password);click(app,'Gerar outro código novo')
            renewed=wait_for('one-time renewal',recovery_code)
            assert renewed!=recovery
            request_close_window(ui,app)
            click(app,'Fechar painel');assert recovery_code()==renewed
            click(app,'Escuro');assert recovery_code()==renewed
            click(app,'Claro');click(app,'Copiar código');wait_text('Código copiado.')
            assert ui.poll() is None,'native close discarded renewal code'
            gi.require_version('Gtk','3.0')
            gi.require_version('Gdk','3.0')
            from gi.repository import Gtk,Gdk
            clipboard=Gtk.Clipboard.get(Gdk.SELECTION_CLIPBOARD)
            copied=clipboard.wait_for_text()
            assert copied==renewed.replace('-',''),'native clipboard differs from newly issued code'
            clipboard.set_text('',-1)
            recovery_actions('renew',renewed)
            click(app,'Guardei meu código');wait_for('security ack closed',lambda:not find(app,'dialog','Segurança'))
            recovery=renewed
            report.append({'step':'BF03 authenticated password renewal/native clipboard/exact content/blocked native close/theme/ack; no secret captured','PASS':True})
            # Repeated company alternation, bounded files/FDs and measured latency.
            measurements['company_switches']=[]
            def descriptors():
                return {str(pid):len(list(Path(f'/proc/{pid}/fd').iterdir())) for pid in (ui.pid,daemon.pid)}
            measurements['fds_before_company_switches']=descriptors()
            company_before=resources.snapshot(ui.pid,daemon.pid)
            measurements['company_memory_before']=company_before
            for i in range(20):
                target='Native Beta' if i%2==0 else 'Native Alpha'
                started=time.monotonic();click(app,'Selecionar empresa');wait_text('Empresas')
                row=wait_for('company row',lambda:next((n for n in nodes(app) if n.get_name()=='Selecionar '+target),None))
                type_label_x11(ui,row,'',activate=True)
                wait_for('company detail',lambda:find(app,'heading',target));click(app,'Abrir empresa');wait_text('Inventário disponível nesta empresa')
                with sqlite3.connect(state/'infradesktop.db') as db:
                    assert db.execute('select c.name from account a join companies c on c.id=a.profile').fetchone()[0]==target
                wait_for('exact visible company brand '+target,lambda:find(app,'push button','Selecionar empresa').get_description()==('Visual Alpha' if target=='Native Alpha' else 'Native Beta'))
                sample=resources.snapshot(ui.pid,daemon.pid)
                measurements['company_switches'].append({'target':target,'seconds_including_automation':time.monotonic()-started,'fds':descriptors(),'PSS_KiB':sample['PSS_sum_KiB'],'process_count':sample['process_count']})
            measurements['fds_after_company_switches']=descriptors()
            company_after=resources.snapshot(ui.pid,daemon.pid)
            measurements['company_memory_after']=company_after
            measurements['company_cpu']=resources.delta(company_before,company_after)
            measurements['memory'].append(resources.snapshot(ui.pid,daemon.pid))
            for _ in range(5):
                before=resources.snapshot(ui.pid,daemon.pid);time.sleep(1);after=resources.snapshot(ui.pid,daemon.pid);measurements['idle'].append(resources.delta(before,after))
            for name,expected in [('Equipamentos','Inventário disponível nesta empresa'),('Topologia','Inventário disponível nesta empresa'),('Abrir configurações','Aparência'),('Dashboard','Inventário disponível nesta empresa')]:
                before=resources.snapshot(ui.pid,daemon.pid);started=time.monotonic();click(app,name);wait_text(expected);after=resources.snapshot(ui.pid,daemon.pid)
                measurements['navigation'].append({'action':name,'response_seconds':time.monotonic()-started,**resources.delta(before,after)})
            measurements['memory'].append(resources.snapshot(ui.pid,daemon.pid));configure_window(ui,1366,768);save('M03-light-1366.png');configure_window(ui)
            close_window(ui,app);assert daemon.poll() is None
            ui,app=open_ui(runtime,client,'Inventário disponível nesta empresa');wait_for('remembered exact company brand',lambda:find(app,'push button','Selecionar empresa').get_description()=='Visual Alpha');assert saved_brand();assert find(app,'push button','Conta: desktop-technical');report.append({'step':'remembered reopen exact Native Alpha/account with daemon alive','PASS':True})
            lifecycle.stop(daemon);wait_text('Reconectando ao serviço');daemon=lifecycle.start(runtime,state)
            wait_for('resume after actual owned daemon restart',lambda:'Serviço ativo' in contents(app) and 'Inventário disponível nesta empresa' in contents(app),budget=20)
            wait_for('remembered exact company brand',lambda:find(app,'push button','Selecionar empresa').get_description()=='Visual Alpha');assert saved_brand();assert find(app,'push button','Conta: desktop-technical')
            report.append({'step':'owned daemon restart and resume exact Native Alpha/account','PASS':True})
            click(app,'Selecionar empresa');wait_text('Empresas')
            beta=wait_for('Native Beta selectable row',lambda:next((n for n in nodes(app) if n.get_name()=='Selecionar Native Beta'),None))
            # A focusable table row is activated by its native accessible action.
            type_label_x11(ui,beta,'',activate=True)
            wait_for('selected Native Beta detail',lambda:find(app,'heading','Native Beta'))
            click(app,'Abrir empresa');wait_text('Inventário disponível nesta empresa')
            with sqlite3.connect(state/'infradesktop.db') as db:assert db.execute('select c.name from account a join companies c on c.id=a.profile').fetchone()[0]=='Native Beta'
            theme('dark',3);click(app,'Conta: desktop-technical');click(app,'Sair');click(app,'Sair');wait_for('revoked login',lambda:find(app,'push button','Entrar'));assert not (client/'remembered').exists();save('M01-dark.png')
            close_window(ui,app);ui,app=open_ui(runtime,client,'Entrar');report.append({'step':'company switch/logout/reopen revoked','PASS':True})
            type_field(ui,app,'Usuário','desktop-technical');type_field(ui,app,'Senha',password)
            box=wait_for('remember checkbox',lambda:find(app,'check box','Manter acesso nesta máquina'));assert box.get_action_iface().do_action(0)
            click(app,'Entrar');wait_text('Inventário disponível nesta empresa');assert not (client/'remembered').exists()
            close_window(ui,app);ui,app=open_ui(runtime,client,'Entrar');report.append({'step':'without remember does not resume','PASS':True})
            click(app,'Recuperar acesso');type_field(ui,app,'Usuário','desktop-technical');type_field(ui,app,'Código de recuperação',recovery);type_field(ui,app,'Nova senha',new_password);one_time_mutation('Redefinir senha')
            rotated=wait_for('rotated one-time recovery',recovery_code);assert rotated!=recovery
            request_close_window(ui,app)
            click(app,'Escuro');assert recovery_code()==rotated
            click(app,'Claro');assert recovery_code()==rotated
            assert ui.poll() is None,'native close discarded reset code'
            report.append({'step':'pending reset/native close/navigation/rotated code survives themes until ack','PASS':True})
            recovery_actions('reset',rotated)
            click(app,'Guardei meu código');wait_for('reset login',lambda:find(app,'push button','Entrar'))
            type_field(ui,app,'Senha',new_password);click(app,'Entrar');wait_text('Inventário disponível nesta empresa');report.append({'step':'recovery/new password/rotation','PASS':True})
            close_window(ui,app);lifecycle.stop(daemon);daemon=lifecycle.start(runtime,state);ui,app=open_ui(runtime,client,'Inventário disponível nesta empresa')
            with sqlite3.connect(state/'infradesktop.db') as db:
                assert db.execute('select count(*) from account').fetchone()[0]==1
                assert db.execute('select count(*) from companies').fetchone()[0]==2
                assert db.execute('select theme from account').fetchone()[0]=='dark'
            measurements['memory'].append(resources.snapshot(ui.pid,daemon.pid));report.append({'step':'durable SQLite account/companies/theme after recovery/restart','PASS':True});close_window(ui,app)
        finally:terminate_ui(ui);lifecycle.stop(daemon)
    (out/'journey.json').write_text(json.dumps({'runtime':'optimized Wails GTK/WebKit on private X11/Xvfb display','technical_isolated':True,'binaries':resources.binary_hashes(lifecycle.ROOT),'steps':report},indent=2)+'\n')
    (out/'resources.json').write_text(json.dumps(measurements,indent=2)+'\n')
    (lifecycle.ROOT/'build/ui-021-desktop/latest.txt').write_text(str(out)+'\n')
    print('BF04 0.4.0 real Wails journey PASS; human acceptance, Wayland and Linux boot/logout NOT RUN')

if __name__=='__main__':main()
