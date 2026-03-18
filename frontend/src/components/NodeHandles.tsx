import { Handle, Position } from '@xyflow/react';

const hidden = { visibility: 'hidden' as const };

export function NodeHandles() {
  return (
    <>
      <Handle id="top-t" type="target" position={Position.Top} style={hidden} />
      <Handle id="top-s" type="source" position={Position.Top} style={hidden} />
      <Handle id="bottom-t" type="target" position={Position.Bottom} style={hidden} />
      <Handle id="bottom-s" type="source" position={Position.Bottom} style={hidden} />
      <Handle id="left-t" type="target" position={Position.Left} style={hidden} />
      <Handle id="left-s" type="source" position={Position.Left} style={hidden} />
      <Handle id="right-t" type="target" position={Position.Right} style={hidden} />
      <Handle id="right-s" type="source" position={Position.Right} style={hidden} />
    </>
  );
}
