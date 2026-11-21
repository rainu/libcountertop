export const bytes = (n: number) => n < 1024 ? `${n} B` : `${(n / 1048576).toFixed(1)} MiB`;
