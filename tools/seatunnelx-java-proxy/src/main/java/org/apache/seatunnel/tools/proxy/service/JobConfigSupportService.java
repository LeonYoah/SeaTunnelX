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

import org.apache.seatunnel.shade.com.typesafe.config.Config;
import org.apache.seatunnel.shade.com.typesafe.config.ConfigException;
import org.apache.seatunnel.shade.com.typesafe.config.ConfigFactory;
import org.apache.seatunnel.shade.com.typesafe.config.ConfigParseOptions;
import org.apache.seatunnel.shade.com.typesafe.config.ConfigResolveOptions;
import org.apache.seatunnel.shade.com.typesafe.config.ConfigSyntax;
import org.apache.seatunnel.shade.com.typesafe.config.impl.Parseable;
import org.apache.seatunnel.shade.org.apache.commons.lang3.StringUtils;

import org.apache.seatunnel.api.common.PluginIdentifier;
import org.apache.seatunnel.api.configuration.ReadonlyConfig;
import org.apache.seatunnel.api.sink.SeaTunnelSink;
import org.apache.seatunnel.api.source.SeaTunnelSource;
import org.apache.seatunnel.api.source.SourceSplit;
import org.apache.seatunnel.api.table.catalog.CatalogTable;
import org.apache.seatunnel.api.table.catalog.Column;
import org.apache.seatunnel.api.table.factory.FactoryUtil;
import org.apache.seatunnel.api.transform.SeaTunnelTransform;
import org.apache.seatunnel.core.starter.utils.ConfigBuilder;
import org.apache.seatunnel.engine.core.parse.ConfigParserUtil;
import org.apache.seatunnel.plugin.discovery.seatunnel.SeaTunnelSinkPluginDiscovery;
import org.apache.seatunnel.plugin.discovery.seatunnel.SeaTunnelSourcePluginDiscovery;
import org.apache.seatunnel.tools.proxy.model.DatasetDag;
import org.apache.seatunnel.tools.proxy.model.JobConfigContext;
import org.apache.seatunnel.tools.proxy.model.NodeKind;
import org.apache.seatunnel.tools.proxy.model.ProxyEdge;
import org.apache.seatunnel.tools.proxy.model.ProxyNode;

import scala.Tuple2;

import java.io.IOException;
import java.io.Serializable;
import java.net.URLClassLoader;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Properties;
import java.util.Set;
import java.util.function.Function;
import java.util.stream.Stream;

import static org.apache.seatunnel.api.options.ConnectorCommonOptions.PLUGIN_INPUT;
import static org.apache.seatunnel.api.options.ConnectorCommonOptions.PLUGIN_NAME;
import static org.apache.seatunnel.api.options.ConnectorCommonOptions.PLUGIN_OUTPUT;
import static org.apache.seatunnel.api.table.factory.FactoryUtil.DEFAULT_ID;

public class JobConfigSupportService {
    public JobConfigContext parseJobContext(Map<String, Object> request) {
        try {
            Config jobConfig = loadJobConfig(request);
            List<Config> sources = getConfigList(jobConfig, "source");
            List<Config> transforms = getConfigList(jobConfig, "transform");
            List<Config> sinks = getConfigList(jobConfig, "sink");

            ConfigParserUtil.checkGraph(sources, transforms, sinks);

            boolean simpleGraph = isSimpleGraph(sources, transforms, sinks);
            List<String> warnings = new ArrayList<>();
            DatasetDag graph =
                    buildGraph(jobConfig, sources, transforms, sinks, simpleGraph, warnings);
            return new JobConfigContext(
                    jobConfig, sources, transforms, sinks, simpleGraph, warnings, graph);
        } catch (ConfigException e) {
            throw new ProxyException(400, "Config parse failed: " + e.getMessage(), e);
        } catch (ProxyException e) {
            throw e;
        } catch (Exception e) {
            throw new ProxyException(400, "Config dag inspection failed: " + e.getMessage(), e);
        }
    }

    public String nodeId(NodeKind kind, int index) {
        return kind.name().toLowerCase() + "-" + index;
    }

    public String getOutputDataset(ReadonlyConfig config) {
        return config.getOptional(PLUGIN_OUTPUT).orElse(DEFAULT_ID);
    }

    public List<String> getInputDatasets(ReadonlyConfig config) {
        return config.getOptional(PLUGIN_INPUT).orElse(Collections.singletonList(DEFAULT_ID));
    }

    private DatasetDag buildGraph(
            Config jobConfig,
            List<Config> sources,
            List<Config> transforms,
            List<Config> sinks,
            boolean simpleGraph,
            List<String> warnings) {
        List<ProxyNode> nodes = new ArrayList<>();
        List<ProxyEdge> edges = new ArrayList<>();
        Map<String, String> producers = new LinkedHashMap<>();
        Map<String, OfficialNodeDisplayInfo> displayPaths =
                resolveOfficialDisplayPaths(jobConfig, sources, transforms, sinks, warnings);

        for (int i = 0; i < sources.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(sources.get(i));
            String nodeId = nodeId(NodeKind.SOURCE, i);
            String output = getOutputDataset(readonlyConfig);
            nodes.add(
                    new ProxyNode(
                            nodeId,
                            NodeKind.SOURCE,
                            readonlyConfig.get(PLUGIN_NAME),
                            i,
                            Collections.emptyList(),
                            output,
                            getNodeDisplayPaths(
                                    displayPaths, nodeId, Collections.singletonList(output)),
                            getNodeDisplayColumns(displayPaths, nodeId)));
            producers.put(output, nodeId);
        }

        for (int i = 0; i < transforms.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(transforms.get(i));
            String nodeId = nodeId(NodeKind.TRANSFORM, i);
            List<String> inputs = getInputDatasets(readonlyConfig);
            String output = getOutputDataset(readonlyConfig);
            nodes.add(
                    new ProxyNode(
                            nodeId,
                            NodeKind.TRANSFORM,
                            readonlyConfig.get(PLUGIN_NAME),
                            i,
                            inputs,
                            output,
                            getNodeDisplayPaths(
                                    displayPaths, nodeId, buildTransformFallback(output, inputs)),
                            getNodeDisplayColumns(displayPaths, nodeId)));
            producers.put(output, nodeId);
        }

        for (int i = 0; i < sinks.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(sinks.get(i));
            String nodeId = nodeId(NodeKind.SINK, i);
            List<String> inputs = getInputDatasets(readonlyConfig);
            nodes.add(
                    new ProxyNode(
                            nodeId,
                            NodeKind.SINK,
                            readonlyConfig.get(PLUGIN_NAME),
                            i,
                            inputs,
                            null,
                            getNodeDisplayPaths(displayPaths, nodeId, inputs),
                            getNodeDisplayColumns(displayPaths, nodeId)));
        }

        if (simpleGraph) {
            if (transforms.isEmpty()) {
                addSimpleEdge(
                        sources.get(0),
                        nodeId(NodeKind.SOURCE, 0),
                        sinks.get(0),
                        nodeId(NodeKind.SINK, 0),
                        warnings,
                        edges);
            } else {
                addSimpleEdge(
                        sources.get(0),
                        nodeId(NodeKind.SOURCE, 0),
                        transforms.get(0),
                        nodeId(NodeKind.TRANSFORM, 0),
                        warnings,
                        edges);
                addSimpleEdge(
                        transforms.get(0),
                        nodeId(NodeKind.TRANSFORM, 0),
                        sinks.get(0),
                        nodeId(NodeKind.SINK, 0),
                        warnings,
                        edges);
            }
            return new DatasetDag(nodes, edges);
        }

        for (int i = 0; i < transforms.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(transforms.get(i));
            String nodeId = nodeId(NodeKind.TRANSFORM, i);
            for (String input : getInputDatasets(readonlyConfig)) {
                edges.add(new ProxyEdge(input, producers.get(input), nodeId));
            }
        }

        for (int i = 0; i < sinks.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(sinks.get(i));
            String nodeId = nodeId(NodeKind.SINK, i);
            for (String input : getInputDatasets(readonlyConfig)) {
                edges.add(new ProxyEdge(input, producers.get(input), nodeId));
            }
        }

        return new DatasetDag(nodes, edges);
    }

    private void addSimpleEdge(
            Config left,
            String leftNodeId,
            Config right,
            String rightNodeId,
            List<String> warnings,
            List<ProxyEdge> edges) {
        ReadonlyConfig leftConfig = ReadonlyConfig.fromConfig(left);
        ReadonlyConfig rightConfig = ReadonlyConfig.fromConfig(right);
        String output = getOutputDataset(leftConfig);
        String input = getInputDatasets(rightConfig).get(0);
        String dataset =
                resolveSimpleDataset(
                        output,
                        input,
                        leftConfig.get(PLUGIN_NAME),
                        rightConfig.get(PLUGIN_NAME),
                        warnings);
        edges.add(new ProxyEdge(dataset, leftNodeId, rightNodeId));
    }

    private String resolveSimpleDataset(
            String output,
            String input,
            String leftPluginName,
            String rightPluginName,
            List<String> warnings) {
        if (StringUtils.equals(output, input)) {
            return output;
        }
        if (DEFAULT_ID.equals(output) && !DEFAULT_ID.equals(input)) {
            warnings.add(
                    String.format(
                            "Simple graph compatibility linked %s to %s by sink/transform input '%s'.",
                            leftPluginName, rightPluginName, input));
            return input;
        }
        if (!DEFAULT_ID.equals(output) && DEFAULT_ID.equals(input)) {
            warnings.add(
                    String.format(
                            "Simple graph compatibility linked %s to %s by source/transform output '%s'.",
                            leftPluginName, rightPluginName, output));
            return output;
        }
        warnings.add(
                String.format(
                        "Simple graph compatibility linked %s(%s) to %s(%s) even though dataset ids differ.",
                        leftPluginName, output, rightPluginName, input));
        return output;
    }

    private Config loadJobConfig(Map<String, Object> request) {
        String content = ProxyRequestUtils.getOptionalString(request, "content");
        String contentFormat = ProxyRequestUtils.getOptionalString(request, "contentFormat");
        String filePath = ProxyRequestUtils.getOptionalString(request, "filePath");
        List<String> variables = ProxyRequestUtils.getStringList(request, "variables");

        if (StringUtils.isNotBlank(content) && StringUtils.isNotBlank(filePath)) {
            throw new ProxyException(400, "Only one of 'content' or 'filePath' can be specified");
        }
        if (StringUtils.isBlank(content) && StringUtils.isBlank(filePath)) {
            throw new ProxyException(400, "Either 'content' or 'filePath' must be provided");
        }

        if (StringUtils.isNotBlank(filePath)) {
            return variables.isEmpty()
                    ? ConfigBuilder.of(filePath)
                    : ConfigBuilder.of(filePath, variables);
        }
        return parseConfigContent(content, variables, contentFormat);
    }

    private Config parseConfigContent(
            String content, List<String> variables, String contentFormat) {
        ConfigResolveOptions resolveOptions =
                ConfigResolveOptions.defaults().setAllowUnresolved(true);
        ConfigParseOptions parseOptions = buildParseOptions(contentFormat);
        Config config = ConfigFactory.parseString(content, parseOptions).resolve(resolveOptions);

        if (!variables.isEmpty()) {
            Properties properties = new Properties();
            for (String variable : variables) {
                if (variable == null) {
                    continue;
                }
                String[] pair = variable.split("=", 2);
                if (pair.length != 2 || StringUtils.isBlank(pair[0])) {
                    throw new ProxyException(
                            400, "Invalid variable, expected key=value: " + variable);
                }
                properties.setProperty(pair[0], pair[1]);
            }
            Config variableConfig =
                    Parseable.newProperties(
                                    properties,
                                    ConfigParseOptions.defaults()
                                            .setOriginDescription("proxy variables"))
                            .parse()
                            .toConfig();
            config = config.resolveWith(variableConfig, resolveOptions);
        }

        return config.resolveWith(ConfigFactory.systemProperties(), resolveOptions);
    }

    private ConfigParseOptions buildParseOptions(String contentFormat) {
        if (StringUtils.isBlank(contentFormat) || "hocon".equalsIgnoreCase(contentFormat)) {
            return ConfigParseOptions.defaults();
        }
        if ("json".equalsIgnoreCase(contentFormat)) {
            return ConfigParseOptions.defaults().setSyntax(ConfigSyntax.JSON);
        }
        throw new ProxyException(
                400, "Unsupported contentFormat: " + contentFormat + ", expected hocon or json");
    }

    private List<Config> getConfigList(Config jobConfig, String path) {
        if (!jobConfig.hasPath(path)) {
            return Collections.emptyList();
        }
        return new ArrayList<>(jobConfig.getConfigList(path));
    }

    private boolean isSimpleGraph(
            List<Config> sources, List<Config> transforms, List<Config> sinks) {
        return sources.size() == 1
                && sinks.size() == 1
                && (transforms.isEmpty() || transforms.size() == 1);
    }

    private Map<String, OfficialNodeDisplayInfo> resolveOfficialDisplayPaths(
            Config jobConfig,
            List<Config> sources,
            List<Config> transforms,
            List<Config> sinks,
            List<String> warnings) {
        Map<String, OfficialNodeDisplayInfo> result = new LinkedHashMap<>();
        Map<String, List<CatalogTable>> datasetCatalogTables = new LinkedHashMap<>();
        ClassLoader originalClassLoader = Thread.currentThread().getContextClassLoader();
        URLClassLoader seatunnelHomeClassLoader = null;
        ClassLoader classLoader = originalClassLoader;
        try {
            seatunnelHomeClassLoader = createSeatunnelHomeClassLoader(originalClassLoader);
            if (seatunnelHomeClassLoader != null) {
                classLoader = seatunnelHomeClassLoader;
                Thread.currentThread().setContextClassLoader(classLoader);
            }

            for (int i = 0; i < sources.size(); i++) {
                Config sourceConfig = sources.get(i);
                ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(sourceConfig);
                String nodeId = nodeId(NodeKind.SOURCE, i);
                String output = getOutputDataset(readonlyConfig);
                try {
                    List<CatalogTable> catalogTables =
                            resolveSourceCatalogTables(readonlyConfig, classLoader);
                    datasetCatalogTables.put(output, catalogTables);
                    result.put(nodeId, toDisplayInfo(catalogTables));
                } catch (Exception e) {
                    throw new ProxyException(
                            400,
                            String.format(
                                    "Source[%d]-%s official tablePath resolution failed: %s",
                                    i, readonlyConfig.get(PLUGIN_NAME), e.getMessage()),
                            e);
                }
            }

            for (int i = 0; i < transforms.size(); i++) {
                Config transformConfig = transforms.get(i);
                ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(transformConfig);
                String nodeId = nodeId(NodeKind.TRANSFORM, i);
                String output = getOutputDataset(readonlyConfig);
                List<CatalogTable> inputCatalogTables =
                        collectInputCatalogTables(readonlyConfig, datasetCatalogTables);
                try {
                    List<CatalogTable> catalogTables =
                            resolveTransformCatalogTables(
                                    readonlyConfig, inputCatalogTables, classLoader);
                    datasetCatalogTables.put(output, catalogTables);
                    result.put(nodeId, toDisplayInfo(catalogTables));
                } catch (Exception e) {
                    throw new ProxyException(
                            400,
                            String.format(
                                    "Transform[%d]-%s official tablePath resolution failed: %s",
                                    i, readonlyConfig.get(PLUGIN_NAME), e.getMessage()),
                            e);
                }
            }

            for (int i = 0; i < sinks.size(); i++) {
                Config sinkConfig = sinks.get(i);
                ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(sinkConfig);
                String nodeId = nodeId(NodeKind.SINK, i);
                List<CatalogTable> inputCatalogTables =
                        collectInputCatalogTables(readonlyConfig, datasetCatalogTables);
                try {
                    result.put(
                            nodeId,
                            resolveSinkDisplayInfo(
                                    readonlyConfig, inputCatalogTables, classLoader));
                } catch (Exception e) {
                    throw new ProxyException(
                            400,
                            String.format(
                                    "Sink[%d]-%s official tablePath resolution failed: %s",
                                    i, readonlyConfig.get(PLUGIN_NAME), e.getMessage()),
                            e);
                }
            }

            return result;
        } finally {
            Thread.currentThread().setContextClassLoader(originalClassLoader);
            PluginClassLoaderUtils.closeQuietly(seatunnelHomeClassLoader);
        }
    }

    URLClassLoader createSeatunnelHomeClassLoader(ClassLoader parent) {
        String seatunnelHome = System.getProperty("SEATUNNEL_HOME");
        if (StringUtils.isBlank(seatunnelHome)) {
            seatunnelHome = System.getenv("SEATUNNEL_HOME");
        }
        if (StringUtils.isBlank(seatunnelHome)) {
            return null;
        }
        Path homePath = Paths.get(seatunnelHome);
        List<String> pluginJars = new ArrayList<>();
        collectJarPaths(homePath.resolve("connectors"), pluginJars);
        collectJarPaths(homePath.resolve("plugins"), pluginJars);
        if (pluginJars.isEmpty()) {
            return null;
        }
        try {
            return PluginClassLoaderUtils.createClassLoader(pluginJars, parent);
        } catch (IOException e) {
            throw new ProxyException(
                    500, "Failed to build SEATUNNEL_HOME plugin classloader: " + e.getMessage(), e);
        }
    }

    void collectJarPaths(Path root, List<String> pluginJars) {
        if (root == null || !Files.exists(root)) {
            return;
        }
        try (Stream<Path> pathStream = Files.walk(root)) {
            pathStream
                    .filter(Files::isRegularFile)
                    .filter(path -> path.getFileName().toString().endsWith(".jar"))
                    .map(Path::toAbsolutePath)
                    .map(Path::toString)
                    .sorted()
                    .forEach(pluginJars::add);
        } catch (IOException e) {
            throw new ProxyException(
                    500,
                    "Failed to scan SEATUNNEL_HOME jars from " + root + ": " + e.getMessage(),
                    e);
        }
    }

    private List<CatalogTable> resolveSourceCatalogTables(
            ReadonlyConfig readonlyConfig, ClassLoader classLoader) {
        String factoryId = readonlyConfig.get(PLUGIN_NAME);
        Function<PluginIdentifier, SeaTunnelSource> fallbackCreateSource =
                pluginIdentifier ->
                        new SeaTunnelSourcePluginDiscovery().createPluginInstance(pluginIdentifier);
        Tuple2<SeaTunnelSource<Object, SourceSplit, Serializable>, List<CatalogTable>> tuple =
                FactoryUtil.createAndPrepareSource(
                        readonlyConfig, classLoader, factoryId, fallbackCreateSource, null);
        return tuple._2();
    }

    private List<CatalogTable> resolveTransformCatalogTables(
            ReadonlyConfig readonlyConfig,
            List<CatalogTable> inputCatalogTables,
            ClassLoader classLoader) {
        SeaTunnelTransform<?> transform =
                FactoryUtil.createAndPrepareMultiTableTransform(
                        inputCatalogTables,
                        readonlyConfig,
                        classLoader,
                        readonlyConfig.get(PLUGIN_NAME));
        List<CatalogTable> producedCatalogTables = transform.getProducedCatalogTables();
        if (producedCatalogTables != null && !producedCatalogTables.isEmpty()) {
            return producedCatalogTables;
        }
        Optional<CatalogTable> singleCatalogTable =
                Optional.ofNullable(transform.getProducedCatalogTable());
        return singleCatalogTable.map(Collections::singletonList).orElse(Collections.emptyList());
    }

    private OfficialNodeDisplayInfo resolveSinkDisplayInfo(
            ReadonlyConfig readonlyConfig,
            List<CatalogTable> inputCatalogTables,
            ClassLoader classLoader) {
        if (inputCatalogTables.isEmpty()) {
            return OfficialNodeDisplayInfo.empty();
        }
        String factoryId = readonlyConfig.get(PLUGIN_NAME);
        Function<PluginIdentifier, SeaTunnelSink> fallbackCreateSink =
                pluginIdentifier ->
                        new SeaTunnelSinkPluginDiscovery().createPluginInstance(pluginIdentifier);
        List<CatalogTable> sinkTables = new ArrayList<>();
        for (CatalogTable catalogTable : inputCatalogTables) {
            SeaTunnelSink<?, ?, ?, ?> sink =
                    FactoryUtil.createAndPrepareSink(
                            catalogTable,
                            readonlyConfig,
                            classLoader,
                            factoryId,
                            fallbackCreateSink,
                            null);
            sink.getWriteCatalogTable().ifPresent(sinkTables::add);
        }
        return toDisplayInfo(sinkTables);
    }

    private List<CatalogTable> collectInputCatalogTables(
            ReadonlyConfig readonlyConfig, Map<String, List<CatalogTable>> datasetCatalogTables) {
        List<CatalogTable> inputCatalogTables = new ArrayList<>();
        for (String inputDataset : getInputDatasets(readonlyConfig)) {
            List<CatalogTable> catalogTables = datasetCatalogTables.get(inputDataset);
            if (catalogTables != null) {
                inputCatalogTables.addAll(catalogTables);
            }
        }
        return inputCatalogTables;
    }

    private OfficialNodeDisplayInfo toDisplayInfo(List<CatalogTable> catalogTables) {
        if (catalogTables == null || catalogTables.isEmpty()) {
            return OfficialNodeDisplayInfo.empty();
        }
        Set<String> tablePaths = new LinkedHashSet<>();
        Map<String, List<String>> columns = new LinkedHashMap<>();
        for (CatalogTable catalogTable : catalogTables) {
            if (catalogTable == null || catalogTable.getTablePath() == null) {
                continue;
            }
            String fullName = catalogTable.getTablePath().getFullName();
            if (StringUtils.isBlank(fullName)) {
                continue;
            }
            tablePaths.add(fullName);
            columns.put(fullName, extractColumnNames(catalogTable));
        }
        return new OfficialNodeDisplayInfo(new ArrayList<>(tablePaths), columns);
    }

    private List<String> getNodeDisplayPaths(
            Map<String, OfficialNodeDisplayInfo> officialPaths,
            String nodeId,
            List<String> fallbackPaths) {
        OfficialNodeDisplayInfo info = officialPaths.get(nodeId);
        if (info != null && !info.getTablePaths().isEmpty()) {
            return info.getTablePaths();
        }
        Set<String> fallback = new LinkedHashSet<>();
        for (String path : fallbackPaths) {
            if (StringUtils.isNotBlank(path)) {
                fallback.add(path);
            }
        }
        return new ArrayList<>(fallback);
    }

    private Map<String, List<String>> getNodeDisplayColumns(
            Map<String, OfficialNodeDisplayInfo> officialPaths, String nodeId) {
        OfficialNodeDisplayInfo info = officialPaths.get(nodeId);
        if (info == null) {
            return Collections.emptyMap();
        }
        return info.getTableColumns();
    }

    private List<String> extractColumnNames(CatalogTable catalogTable) {
        if (catalogTable == null || catalogTable.getTableSchema() == null) {
            return Collections.emptyList();
        }
        List<Column> columns = catalogTable.getTableSchema().getColumns();
        if (columns == null || columns.isEmpty()) {
            return Collections.emptyList();
        }
        List<String> names = new ArrayList<>();
        for (Column column : columns) {
            if (column != null && StringUtils.isNotBlank(column.getName())) {
                names.add(column.getName());
            }
        }
        return names;
    }

    private List<String> buildTransformFallback(String output, List<String> inputs) {
        if (StringUtils.isNotBlank(output)) {
            return Collections.singletonList(output);
        }
        return inputs;
    }

    private static final class OfficialNodeDisplayInfo {
        private final List<String> tablePaths;
        private final Map<String, List<String>> tableColumns;

        private OfficialNodeDisplayInfo(
                List<String> tablePaths, Map<String, List<String>> tableColumns) {
            this.tablePaths = tablePaths == null ? Collections.emptyList() : tablePaths;
            this.tableColumns = tableColumns == null ? Collections.emptyMap() : tableColumns;
        }

        static OfficialNodeDisplayInfo empty() {
            return new OfficialNodeDisplayInfo(Collections.emptyList(), Collections.emptyMap());
        }

        List<String> getTablePaths() {
            return tablePaths;
        }

        Map<String, List<String>> getTableColumns() {
            return tableColumns;
        }
    }
}
