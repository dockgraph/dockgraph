/** Shared layout and visual constants used across components and hooks. */

/** Fixed height for container nodes in the ELK layout. */
export const CONTAINER_NODE_HEIGHT = 84;

/** Fixed height for volume nodes in the ELK layout. */
export const VOLUME_NODE_HEIGHT = 40;

/** Opacity for nodes unrelated to the current selection. */
export const FADE_OPACITY = 0.2;

/** Opacity for edges unrelated to the current selection. */
export const EDGE_FADE_OPACITY = 0.15;

/** Maximum delay between WebSocket reconnection attempts (ms). */
export const RECONNECT_MAX_DELAY = 30_000;

/** Opacity for paused container nodes. */
export const PAUSED_OPACITY = 0.7;

/** Opacity for inactive (exited/created/dead/ghost) nodes. */
export const INACTIVE_OPACITY = 0.5;

/** Diameter of the status indicator dot on container nodes. */
export const STATUS_DOT_SIZE = 6;

/** Disable edge animations and enable lightweight rendering above this node count. */
export const ANIMATION_NODE_LIMIT = 150;

/** Zoom level below which nodes render as simplified colored blocks. */
export const LOD_ZOOM_THRESHOLD = 0.25;

/** React Flow store selector — true when zoom is below the LOD threshold. */
export const zoomSelector = (s: { transform: [number, number, number] }) => s.transform[2] < LOD_ZOOM_THRESHOLD;

/** Hit detection width (px) for canvas edge click targets. */
export const CANVAS_EDGE_HIT_WIDTH = 12;

// --- Edge animation parameters (shared by Canvas and SVG renderers) ---

/** Dot travel speed along depends_on edges (px/s). */
export const DOT_SPEED = 160;

/** Minimum animation duration so short edges stay visible (seconds). */
export const MIN_ANIMATION_DURATION = 1.5;

/** Target spacing between animated dots (px). */
export const DOT_SPACING = 150;

/** Minimum number of animated dots per edge. */
export const MIN_DOTS = 3;

/** Maximum number of animated dots per edge. */
export const MAX_DOTS = 8;

/** Radius of animated dots (px). */
export const DOT_RADIUS = 1.8;

/** Opacity of animated dots. */
export const DOT_OPACITY = 0.6;

/** Radius of edge endpoint circles (px). */
export const ENDPOINT_RADIUS = 2.5;

/** Dash pattern for inactive edges (canvas array form). */
export const DASH_PATTERN = [4, 3] as const;

/** Dash pattern for inactive edges (SVG string form). */
export const DASH_PATTERN_SVG = '4 3';

/** Default edge stroke width. */
export const DEFAULT_EDGE_STROKE_WIDTH = 1;

/** Fallback edge stroke color when no theme color is provided. */
export const DEFAULT_EDGE_STROKE = '#475569';

/** Delay (ms) after the last viewport change before canvas edges redraw. */
export const VIEWPORT_SETTLE_DELAY = 150;

/** Stroke width for highlighted (selected/connected) edges. */
export const HIGHLIGHT_EDGE_STROKE_WIDTH = 2.5;

// --- Detail panel & stats ---

/** Width of the detail panel in pixels. */
export const DETAIL_PANEL_WIDTH = 420;

/** Max number of log lines held in the browser. */
export const LOG_BUFFER_SIZE = 500;

/** Initial number of log lines to fetch. */
export const LOG_TAIL_DEFAULT = 100;

/** CPU % threshold for amber warning. */
export const STATS_CPU_WARN = 60;

/** CPU % threshold for red critical. */
export const STATS_CPU_CRIT = 85;

/** Throttle % threshold for red critical. */
export const STATS_THROTTLE_CRIT = 50;

/** Debounce delay for search input (ms). */
export const SEARCH_DEBOUNCE_MS = 150;
