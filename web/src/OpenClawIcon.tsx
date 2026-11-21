import type { IconProps } from "@tabler/icons-react";
import logo from "./openclaw-logo.svg";
export function OpenClawIcon({ size = 20 }: IconProps) {
  return <img src={logo} width={size} height={size} alt="" aria-hidden="true" />;
}
