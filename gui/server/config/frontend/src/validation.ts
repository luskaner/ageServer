export function validateIPv4(ip: string): boolean {
  if (!ip) return false;
  const ipv4Regex = /^(?:(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9][0-9]?|0)\.){3}(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9][0-9]?|0)$/;
  return ipv4Regex.test(ip.trim());
}

export function validateMulticastIPv4(ip: string): boolean {
  if (!validateIPv4(ip)) return false;
  const firstOctet = Number(ip.trim().split('.')[0]);
  return firstOctet >= 224 && firstOctet <= 239;
}
