# Code Evidence: Java Proxy 多集群

- `agent/internal/installer/runtime_storage_probe.go`:
  - `ensureSeatunnelXJavaProxyService` loops through candidate ports and returns the first healthy endpoint.
  - Current health check alone cannot prove the endpoint belongs to the current `install_dir`.
  - `startSeatunnelXJavaProxyService` passes `-Dseatunnelx.java.proxy.port` and the launcher script adds `-Dseatunnelx.java.proxy.seatunnel.home=${SEATUNNEL_HOME}`.
  - Port candidate order can accept a Control Plane-supplied default port through context and then fall back to state/built-in defaults.
- `internal/config`:
  - `config.yaml` already drives Control Plane runtime behavior such as `app.external_url` and `grpc.port`, so `java_proxy.default_port` belongs here as the single default source.
- `internal/apps/cluster`:
  - Java Proxy lifecycle commands and runtime storage proxy commands are the Control Plane-to-Agent boundary where `java_proxy_default_port` should be attached.
- `agent/internal/installer/seatunnelx_java_proxy_service.go`:
  - Status/stop logic already reads PID/port from `install_dir/.seatunnelx/seatunnelx-java-proxy/`.
  - `seatunnelxJavaProxyPIDsByPort` can discover listener PIDs and is suitable for ownership validation.
- `scripts/seatunnelx-java-proxy.sh`:
  - `kill_existing_proxy_listener` currently kills any `SeatunnelXJavaProxyApplication` on the requested port, regardless of `SEATUNNEL_HOME`.
  - Needs current-install-dir guard for same-host multi-cluster safety.
- `tools/seatunnelx-java-proxy`:
  - Java app accepts `seatunnelx.java.proxy.port` and the server uses Java `HttpServer`.
  - `/healthz` returns only `{ok:true}`, no install_dir or instance identity.
