import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';

export interface Theme {
  mode: 'dark' | 'light';
  // Canvas
  canvasBg: string;
  dotColor: string;
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
  // Controls / UI
  panelBg: string;
  panelBorder: string;
  panelText: string;
  // MiniMap
  minimapBg: string;
  minimapMask: string;
  // Network group
  groupBgAlpha: string;
  groupBorderAlpha: string;
  groupTextAlpha: string;
}

const dark: Theme = {
  mode: 'dark',
  canvasBg: '#0f172a',
  dotColor: '#2d3a4d',
  nodeBg: '#1e293b',
  nodeBorder: '#334155',
  nodeGhostBorder: '#475569',
  nodeText: '#e2e8f0',
  nodeSubtext: '#64748b',
  portBg: '#0f172a',
  portText: '#94a3b8',
  edgeStroke: '#475569',
  panelBg: '#1e293b',
  panelBorder: '#334155',
  panelText: '#94a3b8',
  minimapBg: '#1e293b',
  minimapMask: 'rgba(15, 23, 42, 0.7)',
  groupBgAlpha: '06',
  groupBorderAlpha: '40',
  groupTextAlpha: 'cc',
};

const light: Theme = {
  mode: 'light',
  canvasBg: '#f0ece6',
  dotColor: '#c2bbb0',
  nodeBg: '#f7f5f1',
  nodeBorder: '#d6d0c8',
  nodeGhostBorder: '#b8b0a4',
  nodeText: '#2c2418',
  nodeSubtext: '#7a7060',
  portBg: '#f3f0eb',
  portText: '#5c5448',
  edgeStroke: '#a09888',
  panelBg: '#f7f5f1',
  panelBorder: '#d6d0c8',
  panelText: '#5c5448',
  minimapBg: '#f7f5f1',
  minimapMask: 'rgba(250, 248, 245, 0.7)',
  groupBgAlpha: '0a',
  groupBorderAlpha: '30',
  groupTextAlpha: 'aa',
};

interface ThemeContextValue {
  theme: Theme;
  toggle: () => void;
}

const ThemeContext = createContext<ThemeContextValue>({
  theme: dark,
  toggle: () => {},
});

const STORAGE_KEY = 'docker-flow-theme';

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

export function useTheme() {
  return useContext(ThemeContext);
}
