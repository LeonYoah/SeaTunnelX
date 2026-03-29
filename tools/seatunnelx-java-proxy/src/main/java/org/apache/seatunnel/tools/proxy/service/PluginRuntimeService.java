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

package org.apache.seatunnel.tools.proxy.service;

import org.apache.seatunnel.shade.org.apache.commons.lang3.StringUtils;

import org.apache.seatunnel.api.table.factory.CatalogFactory;
import org.apache.seatunnel.api.table.factory.Factory;
import org.apache.seatunnel.api.table.factory.FactoryUtil;
import org.apache.seatunnel.api.table.factory.TableSinkFactory;
import org.apache.seatunnel.api.table.factory.TableSourceFactory;
import org.apache.seatunnel.api.table.factory.TableTransformFactory;
import org.apache.seatunnel.tools.proxy.model.PluginFactoryInfo;
import org.apache.seatunnel.tools.proxy.model.PluginListResult;

import java.io.IOException;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

public class PluginRuntimeService {

    private static final Map<String, List<PluginFactoryInfo>> LIST_CACHE =
            new ConcurrentHashMap<>();

    public PluginListResult list(Map<String, Object> request) {
        PluginExecutionContext context = openContext(request);
        try {
            String cacheKey = context.getPluginType() + "|" + context.getClasspathFingerprint();
            List<PluginFactoryInfo> plugins = LIST_CACHE.get(cacheKey);
            if (plugins == null) {
                plugins = discoverPluginFactories(context);
                LIST_CACHE.put(cacheKey, plugins);
            }
            return new PluginListResult(
                    true, context.getPluginType(), plugins, context.getWarnings());
        } finally {
            context.close();
        }
    }

    PluginExecutionContext openContext(Map<String, Object> request) {
        String pluginType =
                normalizePluginType(ProxyRequestUtils.getRequiredString(request, "pluginType"));
        List<String> pluginJars = ProxyRequestUtils.getStringList(request, "pluginJars");
        ClassLoader parent = Thread.currentThread().getContextClassLoader();
        URLClassLoader urlClassLoader = null;
        String fingerprint = "classpath";
        List<String> warnings = new ArrayList<>();
        String origin = "classpath";
        try {
            if (!pluginJars.isEmpty()) {
                urlClassLoader = PluginClassLoaderUtils.createClassLoader(pluginJars, parent);
                fingerprint = fingerprint(pluginJars);
                origin = "request_plugin_jars";
            } else {
                urlClassLoader = PluginClassLoaderUtils.createClassLoaderFromSeatunnelHome(parent);
                if (urlClassLoader != null) {
                    String seatunnelHome = System.getProperty("SEATUNNEL_HOME");
                    if (StringUtils.isBlank(seatunnelHome)) {
                        seatunnelHome = System.getenv("SEATUNNEL_HOME");
                    }
                    if (StringUtils.isNotBlank(seatunnelHome)) {
                        List<String> seatunnelHomeJars =
                                PluginClassLoaderUtils.collectJarPaths(Paths.get(seatunnelHome));
                        fingerprint = fingerprint(seatunnelHomeJars);
                    }
                    origin = "seatunnel_home";
                }
            }
        } catch (IOException e) {
            throw new ProxyException(
                    500, "Failed to prepare plugin runtime classloader: " + e.getMessage(), e);
        }
        return new PluginExecutionContext(
                pluginType,
                pluginJars,
                urlClassLoader == null ? parent : urlClassLoader,
                urlClassLoader,
                fingerprint,
                origin,
                warnings);
    }

    Factory discoverFactory(PluginExecutionContext context, String factoryIdentifier) {
        try {
            switch (context.getPluginType()) {
                case "source":
                    return FactoryUtil.discoverFactory(
                            context.getClassLoader(), TableSourceFactory.class, factoryIdentifier);
                case "sink":
                    return FactoryUtil.discoverFactory(
                            context.getClassLoader(), TableSinkFactory.class, factoryIdentifier);
                case "transform":
                    return FactoryUtil.discoverFactory(
                            context.getClassLoader(),
                            TableTransformFactory.class,
                            factoryIdentifier);
                case "catalog":
                    return FactoryUtil.discoverFactory(
                            context.getClassLoader(), CatalogFactory.class, factoryIdentifier);
                default:
                    throw new ProxyException(
                            400, "Unsupported plugin type: " + context.getPluginType());
            }
        } catch (Exception e) {
            throw new ProxyException(
                    400,
                    "Plugin factory not found for type="
                            + context.getPluginType()
                            + ", identifier="
                            + factoryIdentifier
                            + ": "
                            + e.getMessage(),
                    e);
        }
    }

    List<PluginFactoryInfo> discoverPluginFactories(PluginExecutionContext context) {
        List<? extends Factory> factories;
        switch (context.getPluginType()) {
            case "source":
                factories =
                        FactoryUtil.discoverFactories(
                                context.getClassLoader(), TableSourceFactory.class);
                break;
            case "sink":
                factories =
                        FactoryUtil.discoverFactories(
                                context.getClassLoader(), TableSinkFactory.class);
                break;
            case "transform":
                factories =
                        FactoryUtil.discoverFactories(
                                context.getClassLoader(), TableTransformFactory.class);
                break;
            case "catalog":
                factories =
                        FactoryUtil.discoverFactories(
                                context.getClassLoader(), CatalogFactory.class);
                break;
            default:
                throw new ProxyException(
                        400, "Unsupported plugin type: " + context.getPluginType());
        }
        Map<String, PluginFactoryInfo> deduped = new LinkedHashMap<>();
        for (Factory factory : factories) {
            String identifier = StringUtils.trimToEmpty(factory.factoryIdentifier());
            if (StringUtils.isBlank(identifier)) {
                continue;
            }
            deduped.put(
                    identifier.toLowerCase(),
                    new PluginFactoryInfo(
                            identifier, factory.getClass().getName(), context.origin));
        }
        List<PluginFactoryInfo> result = new ArrayList<>(deduped.values());
        result.sort(
                Comparator.comparing(
                        PluginFactoryInfo::getFactoryIdentifier, String.CASE_INSENSITIVE_ORDER));
        return Collections.unmodifiableList(result);
    }

    String resolveCodeSourceFingerprint(Factory factory) {
        try {
            URL url = FactoryUtil.getFactoryUrl(factory);
            if (url == null || !"file".equalsIgnoreCase(url.getProtocol())) {
                return String.valueOf(url);
            }
            return fingerprint(Collections.singletonList(Paths.get(url.toURI()).toString()));
        } catch (Exception e) {
            return factory.getClass().getName();
        }
    }

    String resolveCodeSourceLocation(Factory factory) {
        try {
            URL url = FactoryUtil.getFactoryUrl(factory);
            return url == null ? factory.getClass().getName() : String.valueOf(url);
        } catch (Exception e) {
            return factory.getClass().getName();
        }
    }

    private String normalizePluginType(String pluginType) {
        String normalized = StringUtils.trimToEmpty(pluginType).toLowerCase();
        switch (normalized) {
            case "source":
            case "sink":
            case "transform":
            case "catalog":
                return normalized;
            default:
                throw new ProxyException(400, "Unsupported pluginType: " + pluginType);
        }
    }

    private String fingerprint(List<String> paths) {
        if (paths == null || paths.isEmpty()) {
            return "empty";
        }
        List<String> parts = new ArrayList<>();
        for (String raw : paths) {
            try {
                Path path = Paths.get(raw).toAbsolutePath();
                parts.add(
                        path.toString()
                                + "#"
                                + java.nio.file.Files.size(path)
                                + "#"
                                + java.nio.file.Files.getLastModifiedTime(path).toMillis());
            } catch (Exception e) {
                parts.add(raw);
            }
        }
        Collections.sort(parts);
        return String.join("|", parts);
    }

    static class PluginExecutionContext implements AutoCloseable {
        private final String pluginType;
        private final List<String> pluginJars;
        private final ClassLoader classLoader;
        private final URLClassLoader closeableClassLoader;
        private final String classpathFingerprint;
        private final String origin;
        private final List<String> warnings;

        PluginExecutionContext(
                String pluginType,
                List<String> pluginJars,
                ClassLoader classLoader,
                URLClassLoader closeableClassLoader,
                String classpathFingerprint,
                String origin,
                List<String> warnings) {
            this.pluginType = pluginType;
            this.pluginJars = pluginJars;
            this.classLoader = classLoader;
            this.closeableClassLoader = closeableClassLoader;
            this.classpathFingerprint = classpathFingerprint;
            this.origin = origin;
            this.warnings = warnings;
        }

        String getPluginType() {
            return pluginType;
        }

        List<String> getPluginJars() {
            return pluginJars;
        }

        ClassLoader getClassLoader() {
            return classLoader;
        }

        String getClasspathFingerprint() {
            return classpathFingerprint;
        }

        List<String> getWarnings() {
            return warnings;
        }

        @Override
        public void close() {
            PluginClassLoaderUtils.closeQuietly(closeableClassLoader);
        }
    }
}
