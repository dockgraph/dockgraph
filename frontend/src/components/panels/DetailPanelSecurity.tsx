import { SecurityBadges } from './SecurityBadges';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  security: ContainerDetail['security'];
}

export function DetailPanelSecurity({ security }: Props) {
  if (!security) return null;
  return (
    <SecurityBadges
      privileged={security.privileged}
      readonlyRootfs={security.readonlyRootfs}
      capAdd={security.capAdd}
      capDrop={security.capDrop}
    />
  );
}
