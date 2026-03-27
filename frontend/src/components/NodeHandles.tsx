import { memo } from 'react';
import { Handle, Position } from '@xyflow/react';

const hidden = { visibility: 'hidden' as const };

export const NodeHandles = memo(function NodeHandles() {
  return (
    <>
      <Handle id="t" type="target" position={Position.Top} style={hidden} />
      <Handle id="s" type="source" position={Position.Bottom} style={hidden} />
    </>
  );
});
