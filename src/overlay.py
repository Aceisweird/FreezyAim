#!/usr/bin/env python3
"""Shows the Freezy Aim crosshair on macOS and Linux.

The main app starts this helper. It draws the crosshair to overlay.png and describes
where it goes in overlay.json. This script puts that picture on screen in a
click-through window, and adds a tray or menu bar icon.
"""

from __future__ import annotations

import json
import os
import signal
import sys
from pathlib import Path

MISSING_QT = 3  # The main app shows install instructions when it sees this exit code.
POLL_MS = 500

# Native Wayland apps can't position themselves or stay on top, so use XWayland when
# it's available. This has to be set before Qt starts.
if (
    sys.platform.startswith("linux")
    and os.environ.get("WAYLAND_DISPLAY")
    and os.environ.get("DISPLAY")
):
    os.environ.setdefault("QT_QPA_PLATFORM", "xcb")

try:
    from PySide6.QtCore import Qt, QTimer, QUrl
    from PySide6.QtGui import QDesktopServices, QIcon, QPainter, QPixmap
    from PySide6.QtWidgets import QApplication, QMenu, QSystemTrayIcon, QWidget
except ImportError:
    print("Freezy Aim needs PySide6 to show the crosshair: pip install PySide6", file=sys.stderr)
    sys.exit(MISSING_QT)


def read_state(path: Path) -> dict | None:
    """Read overlay.json, or return None if it isn't ready."""
    try:
        state = json.loads(path.read_text())
    except (OSError, ValueError):
        return None
    return state if isinstance(state, dict) else None


def load_picture(folder: Path, ratio: float) -> QPixmap:
    """Load the crosshair, using the sharper picture on high-resolution screens."""
    if ratio > 1:
        sharp = QPixmap(str(folder / "overlay@2x.png"))
        if not sharp.isNull():
            sharp.setDevicePixelRatio(2)
            return sharp
    return QPixmap(str(folder / "overlay.png"))


def position(area: tuple[int, int, int, int], size: tuple[int, int], state: dict) -> tuple[int, int]:
    """Centre a window of `size` in a screen `area`, then apply the saved nudge."""
    left, top, width, height = area
    return (
        left + (width - size[0]) // 2 + int(state.get("offset_x", 0)),
        top + (height - size[1]) // 2 + int(state.get("offset_y", 0)),
    )


class Overlay(QWidget):
    """A small, transparent, click-through window that stays on top."""

    def __init__(self, folder: Path) -> None:
        super().__init__(
            None,
            Qt.WindowType.FramelessWindowHint
            | Qt.WindowType.WindowStaysOnTopHint
            | Qt.WindowType.Tool  # Keeps it out of the taskbar and app switcher.
            | Qt.WindowType.WindowTransparentForInput  # Clicks pass through.
            | Qt.WindowType.WindowDoesNotAcceptFocus,
        )
        self.setAttribute(Qt.WidgetAttribute.WA_TranslucentBackground)
        self.setAttribute(Qt.WidgetAttribute.WA_TransparentForMouseEvents)
        self.setAttribute(Qt.WidgetAttribute.WA_ShowWithoutActivating)
        self.setAttribute(Qt.WidgetAttribute.WA_MacAlwaysShowToolWindow)

        self.folder = folder
        self.state: dict = {}
        self.stamp: int | None = None
        self.ratio = 0.0
        self.monitors = 0
        self.hidden_from_tray = False
        self.pixmap = QPixmap()

        # This timer also lets Python run often enough to handle Ctrl+C.
        self.timer = QTimer(self)
        self.timer.timeout.connect(self.refresh)
        self.timer.start(POLL_MS)
        self.refresh()

    def refresh(self) -> None:
        self.report_monitors()
        path = self.folder / "overlay.json"
        try:
            stamp = path.stat().st_mtime_ns
        except OSError:
            return
        if stamp != self.stamp:
            state = read_state(path)
            if state is None:
                return  # The file is being replaced, so try again next time.
            self.stamp, self.state = stamp, state
            self.ratio = 0.0  # Loads the new picture below.
            self.setWindowOpacity(float(state.get("opacity", 1)))
        self.place()

    def report_monitors(self) -> None:
        """Tell the main app how many monitors there are for the Settings list."""
        count = len(QApplication.screens())
        if count != self.monitors:
            self.monitors = count
            (self.folder / "overlay_status.json").write_text(json.dumps({"monitors": count}))

    def place(self) -> None:
        screens = QApplication.screens()
        if not screens:
            return
        index = int(self.state.get("screen", 0))
        screen = screens[index] if 0 <= index < len(screens) else screens[0]
        if screen.devicePixelRatio() != self.ratio:
            self.ratio = screen.devicePixelRatio()
            self.pixmap = load_picture(self.folder, self.ratio)
            self.update()
        if self.pixmap.isNull():
            return
        area = screen.geometry()
        scale = self.pixmap.devicePixelRatio()
        size = (round(self.pixmap.width() / scale), round(self.pixmap.height() / scale))
        x, y = position((area.x(), area.y(), area.width(), area.height()), size, self.state)
        self.setGeometry(x, y, *size)
        self.setVisible(bool(self.state.get("visible", True)) and not self.hidden_from_tray)

    def paintEvent(self, _event) -> None:
        if not self.pixmap.isNull():
            QPainter(self).drawPixmap(0, 0, self.pixmap)

    def showEvent(self, event) -> None:
        super().showEvent(event)
        if sys.platform == "darwin":
            QTimer.singleShot(0, self.float_above_macos_fullscreen)

    def float_above_macos_fullscreen(self) -> None:
        """Let the overlay appear over full-screen apps on macOS."""
        try:
            import AppKit
            import objc

            app = AppKit.NSApplication.sharedApplication()
            app.setActivationPolicy_(AppKit.NSApplicationActivationPolicyAccessory)
            window = objc.objc_object(c_void_p=int(self.winId())).window()
            window.setCollectionBehavior_(
                AppKit.NSWindowCollectionBehaviorCanJoinAllSpaces
                | AppKit.NSWindowCollectionBehaviorFullScreenAuxiliary
                | AppKit.NSWindowCollectionBehaviorStationary
            )
            window.setLevel_(AppKit.NSScreenSaverWindowLevel)
        except Exception as error:  # An optional extra shouldn't stop the overlay.
            print(f"Couldn't enable full-screen support: {error}", file=sys.stderr)


def build_tray(overlay: Overlay, url: str) -> QSystemTrayIcon | None:
    if not QSystemTrayIcon.isSystemTrayAvailable():
        print("No system tray found. Press Ctrl+C in the terminal to quit.", file=sys.stderr)
        return None

    menu = QMenu()
    menu.addAction("Open Freezy Aim").triggered.connect(
        lambda: QDesktopServices.openUrl(QUrl(url))
    )
    show_action = menu.addAction("Show crosshair")
    show_action.setCheckable(True)
    show_action.setChecked(True)

    def toggle(on: bool) -> None:
        overlay.hidden_from_tray = not on
        overlay.place()

    show_action.toggled.connect(toggle)
    menu.addSeparator()
    menu.addAction("Quit").triggered.connect(QApplication.quit)

    tray = QSystemTrayIcon(QIcon(str(overlay.folder / "tray.png")))
    tray.setToolTip("Freezy Aim")
    tray.setContextMenu(menu)
    tray.menu = menu  # Qt doesn't keep this alive for us.
    tray.show()
    return tray


def main() -> int:
    if len(sys.argv) != 3:
        print("Start Freezy Aim with the main app, not this helper.", file=sys.stderr)
        return 2
    folder, url = Path(sys.argv[1]), sys.argv[2]

    app = QApplication(sys.argv[:1])
    app.setApplicationName("Freezy Aim")
    app.setQuitOnLastWindowClosed(False)  # Hiding the crosshair shouldn't quit.
    signal.signal(signal.SIGINT, lambda *_: app.quit())
    signal.signal(signal.SIGTERM, lambda *_: app.quit())

    overlay = Overlay(folder)
    tray = build_tray(overlay, url)  # Stored so the icon isn't garbage collected.
    exit_code = app.exec()
    del tray
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
