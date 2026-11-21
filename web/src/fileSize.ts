export function bytes(n: number): string {
 if (!Number.isFinite(n) || n < 0) return "—";
 if (n < 1024) return n + " B";
 const units = ["KiB", "MiB", "GiB", "TiB", "PiB"];
 let value = n / 1024, unit = 0;
 while (value >= 1024 && unit < units.length - 1) { value /= 1024; unit++; }
 return value.toFixed(1) + " " + units[unit];
}
