import React from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../lib/api';
import { 
  Music, 
  Clock, 
  CheckCircle, 
  XCircle, 
  SkipForward,
  RefreshCw,
  Play,
  Pause,
  Youtube
} from 'lucide-react';

interface DashboardStatsCardsProps {
  isPaused: boolean;
  onTogglePause: () => void;
  onRefresh: () => void;
}

const REFETCH_INTERVAL = 60000; // 60 seconds

function StatCard({ 
  title, 
  value, 
  icon: Icon, 
  isLoading = false,
  color = 'blue'
}: {
  title: string;
  value: string | number;
  icon: React.ComponentType<{ className?: string }>;
  isLoading?: boolean;
  color?: 'blue' | 'green' | 'red' | 'yellow' | 'purple' | 'orange';
}) {
  const colorClasses = {
    blue: 'text-blue-600 bg-blue-100',
    green: 'text-green-600 bg-green-100', 
    red: 'text-red-600 bg-red-100',
    yellow: 'text-yellow-600 bg-yellow-100',
    purple: 'text-purple-600 bg-purple-100',
    orange: 'text-orange-600 bg-orange-100',
  };

  return (
    <div className="bg-white overflow-hidden shadow rounded-lg">
      <div className="p-5">
        <div className="flex items-center">
          <div className="flex-shrink-0">
            <div className={`p-3 rounded-md ${colorClasses[color]}`}>
              <Icon className="h-6 w-6" />
            </div>
          </div>
          <div className="ml-5 w-0 flex-1">
            <dt className="text-sm font-medium text-gray-500 truncate">
              {title}
            </dt>
            <dd className="flex items-baseline">
              <div className="text-2xl font-semibold text-gray-900">
                {isLoading ? (
                  <div 
                    className="animate-pulse bg-gray-200 h-8 w-16 rounded"
                    role="status"
                    aria-label="Loading"
                  ></div>
                ) : (
                  value
                )}
              </div>
            </dd>
          </div>
        </div>
      </div>
    </div>
  );
}

function ControlsCard({ isPaused, onTogglePause, onRefresh }: {
  isPaused: boolean;
  onTogglePause: () => void;
  onRefresh: () => void;
}) {
  return (
    <div className="bg-white overflow-hidden shadow rounded-lg">
      <div className="p-5">
        <div className="flex items-center justify-between">
          <div>
            <dt className="text-sm font-medium text-gray-500">
              Auto-refresh
            </dt>
            <dd className="text-xs text-gray-400 mt-1">
              {isPaused ? 'Paused' : '60s interval'}
            </dd>
          </div>
          <div className="flex space-x-2">
            <button
              onClick={onTogglePause}
              className={`inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white focus:outline-none focus:ring-2 focus:ring-offset-2 ${
                isPaused 
                  ? 'bg-green-600 hover:bg-green-700 focus:ring-green-500' 
                  : 'bg-yellow-600 hover:bg-yellow-700 focus:ring-yellow-500'
              }`}
            >
              {isPaused ? (
                <>
                  <Play className="h-4 w-4 mr-1" />
                  Resume
                </>
              ) : (
                <>
                  <Pause className="h-4 w-4 mr-1" />
                  Pause
                </>
              )}
            </button>
            <button
              onClick={onRefresh}
              className="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
            >
              <RefreshCw className="h-4 w-4 mr-1" />
              Refresh
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export function DashboardStatsCards({ isPaused, onTogglePause, onRefresh }: DashboardStatsCardsProps) {
  const queryClient = useQueryClient();
  
  const { data: stats, isLoading, error } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: api.getDashboardStats,
    refetchInterval: isPaused ? false : REFETCH_INTERVAL,
    refetchIntervalInBackground: true,
    staleTime: 30000, // Consider data stale after 30 seconds
  });

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] });
    onRefresh();
  };

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-md p-4">
        <div className="flex">
          <XCircle className="h-5 w-5 text-red-400" />
          <div className="ml-3">
            <h3 className="text-sm font-medium text-red-800">
              Error loading dashboard stats
            </h3>
            <div className="mt-2 text-sm text-red-700">
              <p>Unable to fetch dashboard data. Please try refreshing.</p>
            </div>
            <div className="mt-4">
              <button
                onClick={handleRefresh}
                className="bg-red-100 px-3 py-2 rounded-md text-sm font-medium text-red-800 hover:bg-red-200 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
              >
                Try Again
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
      {/* Controls Card */}
      <ControlsCard 
        isPaused={isPaused}
        onTogglePause={onTogglePause}
        onRefresh={handleRefresh}
      />

      {/* Mappings */}
      <StatCard
        title="Total Mappings"
        value={stats?.mappings.total ?? 0}
        icon={Music}
        isLoading={isLoading}
        color="blue"
      />

      {/* Queue - Pending */}
      <StatCard
        title="Pending Items"
        value={stats?.queue.pending ?? 0}
        icon={Clock}
        isLoading={isLoading}
        color="yellow"
      />

      {/* Queue - Running */}
      <StatCard
        title="Running Items"
        value={stats?.queue.running ?? 0}
        icon={RefreshCw}
        isLoading={isLoading}
        color="blue"
      />

      {/* Queue - Errors */}
      <StatCard
        title="Error Items"
        value={stats?.queue.errors ?? 0}
        icon={XCircle}
        isLoading={isLoading}
        color="red"
      />

      {/* Queue - Skipped */}
      <StatCard
        title="Skipped Items"
        value={stats?.queue.skipped ?? 0}
        icon={SkipForward}
        isLoading={isLoading}
        color="orange"
      />

      {/* Queue - Done */}
      <StatCard
        title="Completed Items"
        value={stats?.queue.done ?? 0}
        icon={CheckCircle}
        isLoading={isLoading}
        color="green"
      />

      {/* YouTube Quota */}
      <StatCard
        title="YouTube Quota"
        value={stats ? `${stats.youtube_quota.used}/${stats.youtube_quota.limit}` : '0/10000'}
        icon={Youtube}
        isLoading={isLoading}
        color="purple"
      />
    </div>
  );
} 