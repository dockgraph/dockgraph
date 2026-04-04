import { useEffect, useRef, useState, useCallback } from "react";
import { useContainerLogs } from "../../hooks/useContainerLogs";
import { formatLogTimestamp } from "../../utils/logParser";
import { useTheme } from "../../theme";

interface Props {
  containerId: string | null;
  active: boolean;
}

export function DetailPanelLogs({ containerId, active }: Props) {
  const { lines, connected, loading, loadingMore, hasMore, loadMore } =
    useContainerLogs(containerId, active);
  const { theme } = useTheme();
  const scrollRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const prevLineCountRef = useRef(0);
  const prevScrollHeightRef = useRef(0);

  // Preserve scroll position when older lines are prepended.
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;

    const prevCount = prevLineCountRef.current;
    const currentCount = lines.length;
    const added = currentCount - prevCount;

    if (added > 0 && prevCount > 0 && !autoScroll) {
      // Lines were prepended — restore scroll position so the view doesn't jump.
      const prevHeight = prevScrollHeightRef.current;
      const newHeight = el.scrollHeight;
      if (newHeight > prevHeight) {
        el.scrollTop += newHeight - prevHeight;
      }
    }

    prevLineCountRef.current = currentCount;
    prevScrollHeightRef.current = el.scrollHeight;
  }, [lines, autoScroll]);

  // Auto-scroll to bottom when new live lines arrive.
  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [lines, autoScroll]);

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;

    const { scrollTop, scrollHeight, clientHeight } = el;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 20);

    // Load more when scrolled near the top (only if content is actually scrollable).
    if (scrollTop < 40 && scrollHeight > clientHeight && hasMore && !loadingMore) {
      prevScrollHeightRef.current = el.scrollHeight;
      loadMore();
    }
  }, [hasMore, loadingMore, loadMore]);

  const jumpToBottom = () => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
    setAutoScroll(true);
  };

  return (
    <div style={{ marginTop: 14 }}>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          marginBottom: 6,
        }}
      >
        <span
          style={{
            fontSize: 11,
            fontWeight: 600,
            color: theme.nodeSubtext,
            textTransform: "uppercase",
            letterSpacing: 0.5,
          }}
        >
          Logs
        </span>
        <span
          style={{
            width: 6,
            height: 6,
            borderRadius: "50%",
            background: connected ? theme.success : theme.danger,
          }}
        />
      </div>
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        style={{
          height: 250,
          overflowY: "auto",
          background: theme.logBg,
          borderRadius: 4,
          padding: 8,
          fontFamily: "monospace",
          fontSize: 10,
          lineHeight: 1.5,
          position: "relative",
        }}
      >
        {loadingMore && (
          <div
            style={{
              textAlign: "center",
              padding: "4px 0 8px",
              color: theme.logMuted,
              fontSize: 10,
            }}
          >
            Loading older logs...
          </div>
        )}
        {!loadingMore && !hasMore && lines.length > 0 && (
          <div
            style={{
              textAlign: "center",
              padding: "4px 0 8px",
              color: theme.logTimestamp,
              fontSize: 10,
            }}
          >
            Beginning of logs
          </div>
        )}
        {loading && lines.length === 0 && (
          <div
            style={{ color: theme.logMuted, textAlign: "center", paddingTop: 20 }}
          >
            Loading logs...
          </div>
        )}
        {!loading && lines.length === 0 && (
          <div
            style={{ color: theme.logMuted, textAlign: "center", paddingTop: 20 }}
          >
            {!active
              ? "Open panel to stream logs"
              : connected
                ? "No log output yet"
                : "Connecting..."}
          </div>
        )}
        {lines.map((line) => (
          <div
            key={line.id}
            style={{ whiteSpace: "pre-wrap", wordBreak: "break-all" }}
          >
            {line.timestamp && (
              <span style={{ color: theme.logTimestamp, marginRight: 6 }}>
                {formatLogTimestamp(line.timestamp)}
              </span>
            )}
            <span
              style={{
                color: line.stream === "stderr" ? theme.danger : theme.logText,
              }}
            >
              {line.text}
            </span>
          </div>
        ))}
      </div>
      {!autoScroll && (
        <button
          onClick={jumpToBottom}
          style={{
            display: "block",
            margin: "4px auto 0",
            fontSize: 10,
            color: theme.panelText,
            background: theme.nodeBg,
            border: `1px solid ${theme.panelBorder}`,
            borderRadius: 4,
            padding: "2px 8px",
            cursor: "pointer",
          }}
        >
          Jump to latest
        </button>
      )}
    </div>
  );
}
