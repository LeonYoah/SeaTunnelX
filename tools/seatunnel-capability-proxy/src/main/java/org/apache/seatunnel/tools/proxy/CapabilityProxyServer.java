/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package org.apache.seatunnel.tools.proxy;

import org.apache.seatunnel.shade.com.fasterxml.jackson.core.type.TypeReference;

import org.apache.seatunnel.common.utils.JsonUtils;
import org.apache.seatunnel.tools.proxy.service.CatalogProbeService;
import org.apache.seatunnel.tools.proxy.service.CheckpointProbeService;
import org.apache.seatunnel.tools.proxy.service.ConfigResourceService;
import org.apache.seatunnel.tools.proxy.service.IMapProbeService;
import org.apache.seatunnel.tools.proxy.service.PreviewConfigService;
import org.apache.seatunnel.tools.proxy.service.ProxyException;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import com.sun.net.httpserver.Headers;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpServer;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class CapabilityProxyServer {

    private static final Logger LOG = LoggerFactory.getLogger(CapabilityProxyServer.class);

    private final HttpServer httpServer;
    private final ExecutorService executorService;
    private final ConfigResourceService configResourceService;
    private final CatalogProbeService catalogProbeService;
    private final CheckpointProbeService checkpointProbeService;
    private final IMapProbeService iMapProbeService;
    private final PreviewConfigService previewConfigService;

    public CapabilityProxyServer(int port, int workerThreads) throws IOException {
        this(
                HttpServer.create(new InetSocketAddress(port), 0),
                Executors.newFixedThreadPool(workerThreads),
                new ConfigResourceService(),
                new CatalogProbeService(),
                new CheckpointProbeService(),
                new IMapProbeService(),
                new PreviewConfigService());
    }

    CapabilityProxyServer(
            HttpServer httpServer,
            ExecutorService executorService,
            ConfigResourceService configResourceService,
            CatalogProbeService catalogProbeService,
            CheckpointProbeService checkpointProbeService,
            IMapProbeService iMapProbeService,
            PreviewConfigService previewConfigService) {
        this.httpServer = httpServer;
        this.executorService = executorService;
        this.configResourceService = configResourceService;
        this.catalogProbeService = catalogProbeService;
        this.checkpointProbeService = checkpointProbeService;
        this.iMapProbeService = iMapProbeService;
        this.previewConfigService = previewConfigService;
        registerContexts();
        this.httpServer.setExecutor(executorService);
    }

    public void start() {
        httpServer.start();
    }

    public void stop(int delaySeconds) {
        httpServer.stop(delaySeconds);
        executorService.shutdownNow();
    }

    private void registerContexts() {
        httpServer.createContext(
                "/healthz",
                exchange -> writeJson(exchange, 200, Collections.singletonMap("ok", true)));
        httpServer.createContext(
                "/api/v1/config/dag",
                new JsonPostHandler() {
                    @Override
                    protected Object handleRequest(Map<String, Object> request) {
                        return configResourceService.inspectDag(request);
                    }
                });
        httpServer.createContext(
                "/api/v1/config/preview/source",
                new JsonPostHandler() {
                    @Override
                    protected Object handleRequest(Map<String, Object> request) {
                        return previewConfigService.deriveSourcePreview(request);
                    }
                });
        httpServer.createContext(
                "/api/v1/config/preview/transform",
                new JsonPostHandler() {
                    @Override
                    protected Object handleRequest(Map<String, Object> request) {
                        return previewConfigService.deriveTransformPreview(request);
                    }
                });
        httpServer.createContext(
                "/api/v1/catalog/probe",
                new JsonPostHandler() {
                    @Override
                    protected Object handleRequest(Map<String, Object> request) {
                        return catalogProbeService.probe(request);
                    }
                });
        httpServer.createContext(
                "/api/v1/storage/checkpoint/probe",
                new JsonPostHandler() {
                    @Override
                    protected Object handleRequest(Map<String, Object> request) {
                        return checkpointProbeService.probe(request);
                    }
                });
        httpServer.createContext(
                "/api/v1/storage/imap/probe",
                new JsonPostHandler() {
                    @Override
                    protected Object handleRequest(Map<String, Object> request) {
                        return iMapProbeService.probe(request);
                    }
                });
    }

    private abstract class JsonPostHandler implements HttpHandler {
        @Override
        public void handle(HttpExchange exchange) throws IOException {
            try {
                if (!"POST".equalsIgnoreCase(exchange.getRequestMethod())) {
                    writeJson(exchange, 405, errorBody("Method not allowed"));
                    return;
                }
                Map<String, Object> request = parseRequest(exchange.getRequestBody());
                Object result = handleRequest(request);
                writeJson(exchange, 200, result);
            } catch (ProxyException e) {
                LOG.warn("Request failed: {}", e.getMessage());
                writeJson(exchange, e.getStatusCode(), errorBody(e.getMessage()));
            } catch (Exception e) {
                LOG.error("Unexpected request failure", e);
                writeJson(exchange, 500, errorBody(e.getMessage()));
            }
        }

        protected abstract Object handleRequest(Map<String, Object> request);
    }

    private Map<String, Object> parseRequest(InputStream requestBody) throws IOException {
        byte[] bytes = readAllBytes(requestBody);
        if (bytes.length == 0) {
            return Collections.emptyMap();
        }
        return JsonUtils.parseObject(
                new String(bytes, StandardCharsets.UTF_8),
                new TypeReference<Map<String, Object>>() {});
    }

    private byte[] readAllBytes(InputStream inputStream) throws IOException {
        byte[] buffer = new byte[4096];
        int bytesRead;
        try (InputStream in = inputStream;
                java.io.ByteArrayOutputStream out = new java.io.ByteArrayOutputStream()) {
            while ((bytesRead = in.read(buffer)) >= 0) {
                out.write(buffer, 0, bytesRead);
            }
            return out.toByteArray();
        }
    }

    private void writeJson(HttpExchange exchange, int statusCode, Object body) throws IOException {
        byte[] payload = JsonUtils.toJsonString(body).getBytes(StandardCharsets.UTF_8);
        Headers headers = exchange.getResponseHeaders();
        headers.add("Content-Type", "application/json; charset=utf-8");
        exchange.sendResponseHeaders(statusCode, payload.length);
        try (OutputStream outputStream = exchange.getResponseBody()) {
            outputStream.write(payload);
        }
    }

    private Map<String, Object> errorBody(String message) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("ok", false);
        body.put("message", message);
        return body;
    }
}
