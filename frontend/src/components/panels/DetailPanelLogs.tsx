import { useEffect, useRef, useState } from "react";
import { useContainerLogs } from "../../hooks/useContainerLogs";
import { formatLogTimestamp } from "../../utils/logParser";
import { useTheme } from "../../theme";

interface Props {
  containerId: string | null;
  active: boolean;
}

export function DetailPanelLogs({ containerId, active }: Props) {
  const { lines, connected } = useContainerLogs(containerId, active);
  const { theme } = useTheme();
  const scrollRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [lines, autoScroll]);

  const handleScroll = () => {
    if (!scrollRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 20);
  };

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
            background: connected ? "#22c55e" : "#ef4444",
          }}
        />
      </div>
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        style={{
          height: 250,
          overflowY: "auto",
          background: "#0f172a",
          borderRadius: 4,
          padding: 8,
          fontFamily: "monospace",
          fontSize: 10,
          lineHeight: 1.5,
          position: "relative",
        }}
      >
        {lines.length === 0 && (
          <div
            style={{ color: "#64748b", textAlign: "center", paddingTop: 20 }}
          >
            {!active
              ? "Open panel to stream logs"
              : connected
                ? "No log output yet"
                : "Connecting..."}
          </div>
        )}
        {lines.map((line, i) => (
          <div
            key={i}
            style={{ whiteSpace: "pre-wrap", wordBreak: "break-all" }}
          >
            {line.timestamp && (
              <span style={{ color: "#475569", marginRight: 6 }}>
                {formatLogTimestamp(line.timestamp)}
              </span>
            )}
            <span
              style={{
                color: line.stream === "stderr" ? "#ef4444" : "#e2e8f0",
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
