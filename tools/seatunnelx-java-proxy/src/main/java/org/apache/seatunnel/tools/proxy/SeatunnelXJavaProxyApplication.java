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

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;

public class SeatunnelXJavaProxyApplication {

    private static final Logger LOG = LoggerFactory.getLogger(SeatunnelXJavaProxyApplication.class);

    private SeatunnelXJavaProxyApplication() {}

    public static void main(String[] args) throws Exception {
        if (args.length > 0 && "probe-once".equalsIgnoreCase(args[0])) {
            int exitCode =
                    new SeatunnelXJavaProxyCli()
                            .run(Arrays.copyOfRange(args, 1, args.length), System.out, System.err);
            if (exitCode != 0) {
                System.exit(exitCode);
            }
            return;
        }
        int port = Integer.parseInt(System.getProperty("seatunnelx.java.proxy.port", "18080"));
        int workers =
                Integer.parseInt(
                        System.getProperty(
                                "seatunnelx.java.proxy.workerThreads",
                                String.valueOf(
                                        Math.max(4, Runtime.getRuntime().availableProcessors()))));
        SeatunnelXJavaProxyServer server = new SeatunnelXJavaProxyServer(port, workers);
        Runtime.getRuntime()
                .addShutdownHook(
                        new Thread(() -> server.stop(0), "seatunnelx-java-proxy-shutdown"));
        server.start();
        LOG.info("SeaTunnel seatunnelx-java-proxy started on port {}", port);
    }
}
