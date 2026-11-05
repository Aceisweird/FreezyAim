# Changelog

## 1.0

The first public release. The sections below are its development builds.

## 1.12.0

### Added

- Draw has Shape, Star, Dot, Select and Pick color tools, filled shapes, dashed and dotted
  lines, three spin mirrors, and buttons to flip, turn and resize.
- Select moves, recolors, resizes, duplicates, reorders or deletes one line.
- Pictures can be rotated, and they take the outline and glow settings.
- Color has a Drawn lines setting that recolors a whole drawing.

### Fixed

- Every editor control now has a visible effect: Length, Gap and Thickness work on crosshairs
  that have no arms yet, and picking a dot shape works with no dot.
- The crosshair color reaches a drawing that has nothing else in it.
- Previews show Opacity.
- A rotated crosshair lines up in Draw, and new lines land where you draw them.
- Large crosshairs stay lined up with the drawing area at high zoom.

## 1.11.1

### Added

- The preview shows center lines while you position an imported picture.

## 1.11.0

### Added

- Draw has a Move tool for moving around the drawing area. Dragging with the middle mouse
  button works with any tool, and Reset view goes back to the middle.
- The Crosshair Editor has an Image tab for using a picture as a crosshair. Drag it in the
  preview or nudge it to move it.

### Changed

- Share codes are much shorter. Older codes can still be imported.
- Settings keeps Nudge and Center crosshair, without the Move right and Move down boxes.

## 1.10.3

### Fixed

- The red X's label shows in front of the window buttons.

## 1.10.2

### Changed

- The crosshair only touches the stacking order when another window has covered it.
- The app runs at below normal priority once it has started, so it can't take time from a game.
- A minimized app window stops drawing.
- WebView2 starts without background network tasks.

### Fixed

- A check for the Edge window's title bar kept running after that window closed.

## 1.10.1

### Fixed

- On Windows, the app's own window draws the page and has no title bar.
- If the page doesn't appear, the app opens in Edge after a few seconds. The tray menu also
  has Open in Edge instead.

## 1.10.0

### Changed

- On Windows, the app opens in its own window using WebView2, so there's no browser title
  bar. If WebView2 can't be used, it opens in an Edge window as before.
- Pages start below the window buttons, and the gap above them moves the window.

## 1.9.2

### Changed

- The minimize, maximize and hide buttons sit in a row next to the red X.

### Fixed

- The title bar's height comes from the browser's own window, so none of it is left showing.

## 1.9.1

### Fixed

- The title bar stays hidden if the browser resets the window's shape.

## 1.9.0

### Added

- On Windows, the title bar is hidden and the app has its own minimize, maximize and hide
  buttons, with a drag area along the top. Settings > Appearance > Hide title bar turns
  this off.

## 1.8.2

### Changed

- The window's title bar now matches your theme and colors.

## 1.8.1

### Changed

- The Home page now says Welcome Back.

### Fixed

- The Home page no longer shows the word "null" after the welcome tips are closed.

## 1.8.0

### Added

- 100 new built-in crosshairs across every category, based on popular styles.

## 1.7.0

### Added

- Draw: a Box tool, an All eight mirror, and a Center button that puts a drawing exactly on
  the aiming point.
- Draw: Outline and Glow controls, with both shown right on the canvas.
- Draw: pen lines are smoothed as you draw. You can turn this off.
- Draw: scroll to zoom, [ and ] to change the line width, and the chosen color is marked.
- New drawings are kept as you go, so an unsaved drawing comes back if the app closes.

### Fixed

- Reopening a saved drawing picks up changes made in the editor, instead of overwriting them.
- Clicking inside a question box no longer cancels it.
- Short messages no longer hide a problem notice when they clear.
- Importing a share code with no name no longer says "Added undefined".
- Page headers and the Draw page fit in narrower windows.
- A damaged draft is dropped instead of breaking the Draw page.
- Moving the mouse while the app loads no longer causes a script error.

## 1.6.2

### Added

- The Draw page shows when a drawing isn't saved yet.

### Fixed

- Clicks and drawing during a page change no longer get lost.

### Changed

- Unsaved drawings now get a warning when you close the window or reload, as well as from
  the red X, and you can save right from it.
- Opening another crosshair in Draw offers to save a new drawing first.

## 1.6.1

### Added

- A share button on every crosshair tile that copies its share code.

### Changed

- New installs start with the Minimal theme and the Bahnschrift font, and Restore default
  settings returns to them.

### Fixed

- Restoring default settings no longer brings back the welcome tips once they're closed.

## 1.6.0

### Added

- Themes in Settings: Color fade, Minimal and Default backgrounds.
- A short guide on the Home page for new users, which you can dismiss.
- The Home page shows which crosshair is in use.
- The editor shows when your changes are saving and saved.
- Icon buttons show their label straight away.
- A list of keyboard shortcuts: press ? or find it in Settings > App.
- A Draw your own button on the Crosshair Editor's start page.

### Changed

- The red X asks first if you have an unsaved drawing.

## 1.4.0

### Added

- Draw, under Crosshair editor: draw a crosshair by hand with a pen, lines, circles and an
  eraser. You can mirror what you draw, snap it to pixels, and draw on top of any crosshair.
- Merge, a new tab in the Crosshair Editor: stack up to three other crosshairs on one.
  Each keeps its own look, and can be moved, faded or placed behind.
- Search and a Favorites filter on My crosshairs and the Crosshair Browser.
- Share codes: copy a crosshair as a code, and import codes from other people.
- Back up all your crosshairs to a file, and restore them from it.
- A Show crosshair switch on the Home page.
- Nudge buttons that move the crosshair one pixel at a time.
- Keyboard shortcuts: Ctrl+Z and Ctrl+Y for Undo and Redo, / or Ctrl+F to search, and
  P, L, C and E to pick a drawing tool.

### Changed

- The editor's preview stays in view while you scroll through a long list of options.
- Picking a chip in the editor is now always its own step for Undo.

## 1.3.0

### Added

- A new opening: sight lines close in on the Home logo, which draws itself and locks on.
- Pages change like a scope pulling focus, and the sidebar keeps gliding while they do.
- Corner brackets lock onto a crosshair as you point at it, and its preview leans toward
  your pointer.
- A ring closes on a crosshair when you choose it, and the tick and star animate only on
  the one that changed.
- Tabs slide in from the side they sit on, and cards ease to their new height.
- Questions before deleting or restoring now appear in the app instead of a browser box.
- The app fades away when you close it with the red X.

### Changed

- Buttons, tiles, tabs, switches and the sidebar now move on spring curves, and they
  press in quickly and spring back.
- Theme colors melt into each other, and the sidebar's colors fade as you move around.
- The mouse wheel eases rails along smoothly. Rails no longer snap to a tile when they stop.
- Previews fade in as they load, and fonts load early so text doesn't shift at startup.

### Fixed

- A press at the very edge of a button no longer gets lost when the button shrinks.

## 1.2.4

### Changed

- The Home page logo is now a simple reticle with the app's name, and the name's color
  follows your theme. A `logo.png` in the data folder still replaces it.

## 1.2.3

### Changed

- Crosshairs and previews draw much faster and use less memory, so editing feels quicker,
  especially at high zoom. They look exactly the same as before.
- Large pages redraw faster.

## 1.2.2

### Added

- Category and crosshair names in the sidebar grow slightly when you hover over them.
- The sidebar highlight, tabs and option buttons fill with color from the left when selected.
- Sidebar icons and crosshair previews react when you hover over them.

### Changed

- Gradient buttons now fade from the accent color into the main color, and their text color
  is chosen to stay readable across the fade.

## 1.2.1

### Changed

- Crosshair previews are drawn at full resolution instead of being stretched, so they stay
  sharp at every zoom level.
- macOS and Linux show a sharper crosshair on high-resolution screens.
- The red X is smaller, and its label only appears while you hover over it.

### Fixed

- The red X now always ends the app and removes the crosshair, even if the normal shutdown
  gets stuck.
- Copies from before the rename to Freezy Aim now close when the app starts, so they can't
  leave a second crosshair on screen.

## 1.2.0

### Added

- Red X button at the top right that closes the whole app.
- macOS and Linux get the full app window, editor and all crosshairs. A small helper
  shows the crosshair on screen and adds a tray or menu bar icon.
- New original crosshairs in the Scopes, Frames, Dots, Classic and Bold sections.

### Removed

- All picture crosshairs, the Pictures editor bases and the Fun section. Every crosshair
  is now drawn by the app itself.

### Changed

- The project is now organised for all three systems, with the app's code in `src/`.

## 1.1.0

### Fixed

- Opening a new version now closes any other version still running. Before, it could
  reopen the old window instead.
- Every sidebar section stays on screen in short windows, and only the category list
  scrolls.
- Link addresses no longer pop up when you hover over buttons.

### Added

- Font choice for all text except the app name in the sidebar.
- Settings tabs for Appearance, Crosshair, Shortcuts and App.
- Text size and an Animations switch.
- Shortcuts to show or hide the crosshair and to switch to the next favorite.
- Start with Windows, and an option to start quietly in the tray.
- Center crosshair and Restore default settings.

### Changed

- Smaller download and repository: fonts and images are compressed, and web features the
  app doesn't use are left out.
- Less background work while you play, and much less data sent to the app window.
- The browser's right-click menu no longer appears in the app.

## 1.0.0

First public release.

### App

- Click-through crosshair overlay that stays on top and never takes focus.
- Home page with recent crosshairs and favorites.
- My Crosshairs for choosing, duplicating and deleting your own crosshairs.
- Crosshair Browser with built-in crosshairs sorted into sections.
- Crosshair Editor with 30 bases and options for arms, frames, dots, colors,
  outline, opacity, glow and rotation, plus Undo, Redo and a live preview.
- Picture crosshairs, including full-color ones.
- Main and accent colors in Settings.
- Tray icon to reopen the window, show or hide the crosshair, or quit.

### Windows

- Settings now live in `%APPDATA%\Freezy Aim`. Files from earlier builds are copied
  there the first time you run this version.
- The `.exe` has its own icon and version details.
- Problems are written to `freezy-aim.log` in the same folder.
