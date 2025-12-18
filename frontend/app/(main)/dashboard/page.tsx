'use client';

import {motion} from 'motion/react';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {
  Server,
  Database,
  Activity,
  CheckCircle,
  AlertCircle,
  Clock,
  Cpu,
  HardDrive,
  MemoryStick,
  Ship,
} from 'lucide-react';

// 模拟数据 - 后续接入真实 API
const stats = [
  {
    title: '主机总数',
    value: '12',
    icon: Server,
    description: '已注册主机',
    trend: '+2 本周',
    color: 'text-blue-500',
    bgColor: 'bg-blue-500/10',
  },
  {
    title: '集群数量',
    value: '3',
    icon: Database,
    description: '运行中集群',
    trend: '全部正常',
    color: 'text-green-500',
    bgColor: 'bg-green-500/10',
  },
  {
    title: '在线 Agent',
    value: '10',
    icon: Activity,
    description: '活跃连接',
    trend: '2 离线',
    color: 'text-cyan-500',
    bgColor: 'bg-cyan-500/10',
  },
  {
    title: '今日任务',
    value: '156',
    icon: CheckCircle,
    description: '已完成任务',
    trend: '成功率 98%',
    color: 'text-purple-500',
    bgColor: 'bg-purple-500/10',
  },
];

const recentActivities = [
  {
    id: 1,
    type: 'success',
    message: '集群 prod-cluster-01 启动成功',
    time: '2 分钟前',
  },
  {
    id: 2,
    type: 'info',
    message: 'Agent node-03 已重新连接',
    time: '15 分钟前',
  },
  {
    id: 3,
    type: 'warning',
    message: '主机 worker-05 CPU 使用率超过 80%',
    time: '1 小时前',
  },
  {
    id: 4,
    type: 'success',
    message: '数据同步任务 sync-mysql-to-hive 完成',
    time: '2 小时前',
  },
  {
    id: 5,
    type: 'info',
    message: '新主机 worker-06 已注册',
    time: '3 小时前',
  },
];

const clusterStatus = [
  {
    name: 'prod-cluster-01',
    status: 'running',
    nodes: 5,
    cpu: 45,
    memory: 62,
    disk: 38,
  },
  {
    name: 'dev-cluster-01',
    status: 'running',
    nodes: 3,
    cpu: 23,
    memory: 41,
    disk: 25,
  },
  {
    name: 'test-cluster-01',
    status: 'stopped',
    nodes: 2,
    cpu: 0,
    memory: 0,
    disk: 15,
  },
];

export default function DashboardPage() {
  return (
    <div className='space-y-6'>
      {/* 页面标题 */}
      <motion.div
        initial={{opacity: 0, y: -20}}
        animate={{opacity: 1, y: 0}}
        transition={{duration: 0.5}}
        className='flex items-center gap-3'
      >
        <Ship className='w-8 h-8 text-cyan-500' />
        <div>
          <h1 className='text-2xl font-bold'>控制台</h1>
          <p className='text-muted-foreground text-sm'>
            SeaTunnelX 运维管理概览
          </p>
        </div>
      </motion.div>

      {/* 统计卡片 */}
      <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4'>
        {stats.map((stat, index) => (
          <motion.div
            key={stat.title}
            initial={{opacity: 0, y: 20}}
            animate={{opacity: 1, y: 0}}
            transition={{delay: index * 0.1, duration: 0.5}}
          >
            <Card>
              <CardHeader className='flex flex-row items-center justify-between pb-2'>
                <CardTitle className='text-sm font-medium text-muted-foreground'>
                  {stat.title}
                </CardTitle>
                <div className={`p-2 rounded-lg ${stat.bgColor}`}>
                  <stat.icon className={`w-4 h-4 ${stat.color}`} />
                </div>
              </CardHeader>
              <CardContent>
                <div className='text-2xl font-bold'>{stat.value}</div>
                <p className='text-xs text-muted-foreground mt-1'>
                  {stat.trend}
                </p>
              </CardContent>
            </Card>
          </motion.div>
        ))}
      </div>

      <div className='grid grid-cols-1 lg:grid-cols-2 gap-6'>
        {/* 集群状态 */}
        <motion.div
          initial={{opacity: 0, x: -20}}
          animate={{opacity: 1, x: 0}}
          transition={{delay: 0.4, duration: 0.5}}
        >
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <Database className='w-5 h-5 text-cyan-500' />
                集群状态
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='space-y-4'>
                {clusterStatus.map((cluster) => (
                  <div
                    key={cluster.name}
                    className='p-4 rounded-lg bg-muted/50 space-y-3'
                  >
                    <div className='flex items-center justify-between'>
                      <div className='flex items-center gap-2'>
                        <span
                          className={`w-2 h-2 rounded-full ${
                            cluster.status === 'running'
                              ? 'bg-green-500'
                              : 'bg-gray-400'
                          }`}
                        />
                        <span className='font-medium'>{cluster.name}</span>
                      </div>
                      <span className='text-sm text-muted-foreground'>
                        {cluster.nodes} 节点
                      </span>
                    </div>
                    {cluster.status === 'running' && (
                      <div className='grid grid-cols-3 gap-4 text-sm'>
                        <div className='flex items-center gap-2'>
                          <Cpu className='w-4 h-4 text-blue-500' />
                          <span>CPU {cluster.cpu}%</span>
                        </div>
                        <div className='flex items-center gap-2'>
                          <MemoryStick className='w-4 h-4 text-purple-500' />
                          <span>内存 {cluster.memory}%</span>
                        </div>
                        <div className='flex items-center gap-2'>
                          <HardDrive className='w-4 h-4 text-orange-500' />
                          <span>磁盘 {cluster.disk}%</span>
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* 最近活动 */}
        <motion.div
          initial={{opacity: 0, x: 20}}
          animate={{opacity: 1, x: 0}}
          transition={{delay: 0.4, duration: 0.5}}
        >
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <Activity className='w-5 h-5 text-cyan-500' />
                最近活动
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='space-y-4'>
                {recentActivities.map((activity) => (
                  <div
                    key={activity.id}
                    className='flex items-start gap-3 pb-3 border-b last:border-0 last:pb-0'
                  >
                    <div className='mt-0.5'>
                      {activity.type === 'success' && (
                        <CheckCircle className='w-4 h-4 text-green-500' />
                      )}
                      {activity.type === 'warning' && (
                        <AlertCircle className='w-4 h-4 text-yellow-500' />
                      )}
                      {activity.type === 'info' && (
                        <Activity className='w-4 h-4 text-blue-500' />
                      )}
                    </div>
                    <div className='flex-1 min-w-0'>
                      <p className='text-sm'>{activity.message}</p>
                      <p className='text-xs text-muted-foreground flex items-center gap-1 mt-1'>
                        <Clock className='w-3 h-3' />
                        {activity.time}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </motion.div>
      </div>

      {/* 快速操作提示 */}
      <motion.div
        initial={{opacity: 0, y: 20}}
        animate={{opacity: 1, y: 0}}
        transition={{delay: 0.6, duration: 0.5}}
      >
        <Card className='bg-gradient-to-r from-cyan-500/10 to-blue-500/10 border-cyan-500/20'>
          <CardContent className='py-4'>
            <div className='flex items-center justify-between'>
              <div className='flex items-center gap-3'>
                <Ship className='w-6 h-6 text-cyan-500' />
                <div>
                  <p className='font-medium'>欢迎使用 SeaTunnelX</p>
                  <p className='text-sm text-muted-foreground'>
                    开始添加主机并部署您的第一个 SeaTunnel 集群
                  </p>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </motion.div>
    </div>
  );
}
