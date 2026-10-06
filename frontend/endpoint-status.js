const labels = {
  operational: ["Available", "state--ready"],
  degraded: ["Degraded", "state--warning"],
  unavailable: ["Unavailable", "state--unavailable"],
  unknown: ["Status unknown", "state--choice"],
};

export function endpointFreshUntil(value) {
  if (value instanceof Date) return Number.isFinite(value.getTime()) ? value.toISOString() : "";
  return typeof value === "string" ? value.slice(0, 50) : "";
}

export function endpointStatus(endpoint, now = Date.now()) {
  const until = Date.parse(endpoint?.fresh_until ?? endpoint?.freshUntil ?? "");
  const stale = endpoint?.freshness === "stale" || (Number.isFinite(until) && until <= now);
  // A current policy denial remains meaningful even without a health probe.
  const denied = endpoint?.callable === false && endpoint?.status === "unavailable";
  const fresh = endpoint?.freshness === "fresh" && Number.isFinite(until) && until > now;
  const status = !fresh && !denied ? "unknown" : (labels[endpoint?.status] ? endpoint.status : "unknown");
  const [label, className] = labels[status];
  const detail = stale && !denied ? "Health evidence is out of date" :
    typeof endpoint?.status_detail === "string" ? endpoint.status_detail.slice(0, 180) :
    typeof endpoint?.statusDetail === "string" ? endpoint.statusDetail.slice(0, 180) : "Waiting for endpoint status";
  return { label, className, detail, status };
}
