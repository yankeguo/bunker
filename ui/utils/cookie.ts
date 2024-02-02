export function extractCookie(s: string, name: string): string | undefined {
  const splits = s.split(";");
  for (let split of splits) {
    split = split.trim();
    if (split.startsWith(name + "=")) {
      return split.substring(name.length + 1);
    }
  }
  return;
}
