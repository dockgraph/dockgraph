import { useCallback, useRef } from 'react';
import { Z } from '../../utils/constants';
import type { LogWindowsController } from '../../hooks/useLogWindows';
import { LogWindow } from './LogWindow';
import { LogDock } from './LogDock';

interface Props {
  controller: LogWindowsController;
  /** Opens the side info panel for a container (wired to useDetailPanel). */
  onOpenInfo: (containerId: string) => void;
}

/** Title+tab-bar drop zone height (px) used for merge hit-testing. */
const MERGE_ZONE_H = 40;

export function LogWindowLayer({ controller, onOpenInfo }: Props) {
  const {
    windows, flashId, closeWindow, closeTab, focusWindow, move, resize,
    minimize, restore, setActiveTab, setSearch, merge, detachTab,
  } = controller;

  // Tracks the live pointer position of the window currently being dragged.
  const dragPointer = useRef<{ x: number; y: number } | null>(null);

  const handleDragMove = useCallback((_id: string, clientX: number, clientY: number) => {
    dragPointer.current = { x: clientX, y: clientY };
  }, []);

  const handleDragEnd = useCallback(
    (sourceId: string) => {
      const p = dragPointer.current;
      dragPointer.current = null;
      if (!p) return;
      // Find a different, visible window whose title/tab-bar zone is under the pointer.
      const target = windows.find((w) => {
        if (w.id === sourceId || w.minimized) return false;
        return p.x >= w.x && p.x <= w.x + w.w && p.y >= w.y && p.y <= w.y + MERGE_ZONE_H;
      });
      if (target) merge(sourceId, target.id);
    },
    [windows, merge],
  );

  const visible = windows.filter((w) => !w.minimized);
  const minimized = windows.filter((w) => w.minimized);

  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: Z.logWindowBase, pointerEvents: 'none' }}>
      {visible.map((w) => (
        <div key={w.id} style={{ position: 'absolute', zIndex: w.z }}>
          <LogWindow
            win={w}
            flashed={flashId === w.id}
            onFocus={focusWindow}
            onClose={closeWindow}
            onCloseTab={closeTab}
            onMinimize={minimize}
            onMove={move}
            onResize={resize}
            onSetActiveTab={setActiveTab}
            onSetSearch={setSearch}
            onTitleClick={onOpenInfo}
            onDragMove={handleDragMove}
            onDragEnd={handleDragEnd}
            onTabDetach={detachTab}
          />
        </div>
      ))}
      <LogDock minimized={minimized} onRestore={restore} />
    </div>
  );
}

export default LogWindowLayer;
