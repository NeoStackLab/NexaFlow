"use client";

import { useEffect, useState } from "react";
import { Activity, Database, Server } from "lucide-react";

import { apiClient } from "@/lib/api/client";

type HealthStatus = {
  status: "ok" | "degraded";
  service: string;
  version: string;
  dependencies: Record<string, "ok" | "unavailable">;
};

type ApiResponse<T> = {
  code: number;
  message: string;
  data: T;
};

type ViewState =
  | { kind: "loading" }
  | { kind: "available"; health: HealthStatus }
  | { kind: "unavailable" };

export function SystemStatus() {
  const [state, setState] = useState<ViewState>({ kind: "loading" });

  useEffect(() => {
    const controller = new AbortController();

    apiClient
      .get<ApiResponse<HealthStatus>>("/health", { signal: controller.signal })
      .then(({ data }) => setState({ kind: "available", health: data.data }))
      .catch(() => {
        if (!controller.signal.aborted) {
          setState({ kind: "unavailable" });
        }
      });

    return () => controller.abort();
  }, []);

  const isHealthy = state.kind === "available" && state.health.status === "ok";
  const label =
    state.kind === "loading"
      ? "正在连接 / Connecting"
      : isHealthy
        ? "基础服务正常 / Operational"
        : "等待基础设施 / Infrastructure offline";

  return (
    <section
      className="status-panel reveal-delay-2"
      aria-live="polite"
      aria-label="NexaFlow system status"
    >
      <div className="flex items-center gap-3">
        <span
          className={`status-dot ${isHealthy ? "status-dot-online" : "status-dot-pending"}`}
          aria-hidden="true"
        />
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-ink-muted">
            Runtime status
          </p>
          <p className="mt-1 text-sm font-semibold text-ink">{label}</p>
        </div>
      </div>

      <div className="mt-5 grid grid-cols-3 gap-2">
        {[
          { icon: Server, label: "API", key: "api" },
          { icon: Database, label: "Postgres", key: "postgres" },
          { icon: Activity, label: "Redis", key: "redis" },
        ].map(({ icon: Icon, label: itemLabel, key }) => {
          const online =
            state.kind === "available" &&
            (key === "api" || state.health.dependencies[key] === "ok");

          return (
            <div className="dependency-chip" key={key}>
              <Icon className="size-3.5" aria-hidden="true" />
              <span>{itemLabel}</span>
              <span
                className={`ml-auto size-1.5 rounded-full ${online ? "bg-signal" : "bg-amber-500"}`}
              />
            </div>
          );
        })}
      </div>
    </section>
  );
}
