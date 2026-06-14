import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';

export interface Theme {
  mode: 'dark' | 'light';
  // Canvas
  canvasBg: string;
  dotColor: string;
  dotColorMajor: string;
  // Nodes
  nodeBg: string;
  nodeBorder: string;
  nodeGhostBorder: string;
  nodeText: string;
  nodeSubtext: string;
  portBg: string;
  portText: string;
  // Edges
  edgeStroke: string;
  edgeSignal: string;
  // Signature accent
  accent: string;
  accentSoft: string;
  // Controls / UI
  panelBg: string;
  panelBorder: string;
  panelText: string;
  rowHover: string;
  cardBg: string;
  statsRowBg: string;
  // MiniMap
  minimapBg: string;
  minimapMask: string;
  // Network group
  groupBgAlpha: string;
  groupBorderAlpha: string;
  groupTextAlpha: string;
  // Semantic status
  danger: string;
  warning: string;
  success: string;
  info: string;
  // Log viewer
  logBg: string;
  logText: string;
  logMuted: string;
  logTimestamp: string;
  // Floating log windows
  windowBg: string;
  windowBorder: string;
  windowShadow: string;
  tabActiveBg: string;
  dropIndicator: string;
}

// Technical Blueprint — deep slate-navy canvas, fine grid, teal signal accent.
const dark: Theme = {
  mode: 'dark',
  canvasBg: '#0d1420',
  dotColor: 'rgba(122, 162, 204, 0.10)',
  dotColorMajor: 'rgba(122, 162, 204, 0.22)',
  nodeBg: '#131e30',
  nodeBorder: '#213046',
  nodeGhostBorder: '#2f425c',
  nodeText: '#e6edf5',
  nodeSubtext: '#7889a0',
  portBg: '#0f1929',
  portText: '#8aa0b5',
  edgeStroke: '#365070',
  edgeSignal: '#1f9488',
  accent: '#2dd4bf',
  accentSoft: 'rgba(45, 212, 191, 0.12)',
  panelBg: '#101928',
  panelBorder: '#213046',
  panelText: '#8aa0b5',
  rowHover: '#172538',
  cardBg: '#111c2c',
  statsRowBg: 'rgba(56, 189, 248, 0.05)',
  minimapBg: '#101928',
  minimapMask: 'rgba(13, 20, 32, 0.7)',
  groupBgAlpha: '12',
  groupBorderAlpha: '66',
  groupTextAlpha: 'f0',
  danger: '#f87171',
  warning: '#fbbf24',
  success: '#34d399',
  info: '#38bdf8',
  logBg: '#0d141d',
  logText: '#e6edf5',
  logMuted: '#5a7088',
  logTimestamp: '#465d75',
  windowBg: '#0f1828',
  windowBorder: '#243450',
  windowShadow: '0 12px 40px rgba(0, 0, 0, 0.55)',
  tabActiveBg: '#172538',
  dropIndicator: '#2dd4bf',
};

const light: Theme = {
  mode: 'light',
  canvasBg: '#f0ece6',
  dotColor: 'rgba(120, 100, 70, 0.14)',
  dotColorMajor: 'rgba(120, 100, 70, 0.28)',
  nodeBg: '#f7f5f1',
  nodeBorder: '#d6d0c8',
  nodeGhostBorder: '#b8b0a4',
  nodeText: '#2c2418',
  nodeSubtext: '#7a7060',
  portBg: '#f3f0eb',
  portText: '#5c5448',
  edgeStroke: '#a09888',
  edgeSignal: '#0d9488',
  accent: '#0d9488',
  accentSoft: 'rgba(13, 148, 136, 0.10)',
  panelBg: '#f7f5f1',
  panelBorder: '#d6d0c8',
  panelText: '#5c5448',
  rowHover: '#e8e2d9',
  cardBg: '#f7f5f1',
  statsRowBg: 'rgba(0, 0, 0, 0.03)',
  minimapBg: '#f7f5f1',
  minimapMask: 'rgba(250, 248, 245, 0.7)',
  groupBgAlpha: '0f',
  groupBorderAlpha: '3a',
  groupTextAlpha: 'b2',
  danger: '#dc2626',
  warning: '#d97706',
  success: '#16a34a',
  info: '#2563eb',
  logBg: '#f1f5f9',
  logText: '#1e293b',
  logMuted: '#94a3b8',
  logTimestamp: '#64748b',
  windowBg: '#ffffff',
  windowBorder: '#d6d0c8',
  windowShadow: '0 12px 40px rgba(60, 50, 30, 0.22)',
  tabActiveBg: '#e8e2d9',
  dropIndicator: '#0d9488',
};

interface ThemeContextValue {
  theme: Theme;
  toggle: () => void;
}

const ThemeContext = createContext<ThemeContextValue>({
  theme: dark,
  toggle: () => {},
});

const STORAGE_KEY = 'dockgraph-theme';

function getInitialMode(): 'dark' | 'light' {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === 'light' || stored === 'dark') return stored;
  } catch { /* ignore */ }
  return 'dark';
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<'dark' | 'light'>(getInitialMode);

  const toggle = useCallback(() => {
    setMode((prev) => {
      const next = prev === 'dark' ? 'light' : 'dark';
      try { localStorage.setItem(STORAGE_KEY, next); } catch { /* ignore */ }
      return next;
    });
  }, []);

  const theme = mode === 'dark' ? dark : light;

  return (
    <ThemeContext.Provider value={{ theme, toggle }}>
      {children}
    </ThemeContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTheme() {
  return useContext(ThemeContext);
}
