import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export const ServerMetricsTypes = ['cpu', 'disk', 'network'] as const;
export const LoadBalancerMetricsTypes = [
  'open-connections',
  'connections-per-second',
  'requests-per-second',
  'bandwidth',
] as const;

export interface Query extends DataQuery {
  queryType: 'resource-list' | 'metrics';
  resourceType: 'server' | 'load-balancer';
  metricsType: (typeof ServerMetricsTypes)[number] | (typeof LoadBalancerMetricsTypes)[number];

  selectBy: 'label' | 'id';
  labelSelectors: string[];
  resourceIDs: number[];
}

export const DEFAULT_QUERY: Partial<Query> = {
  queryType: 'metrics',
  resourceType: 'server',
  metricsType: 'cpu',
  selectBy: 'label',
  labelSelectors: [],
  resourceIDs: [],
};

export const DEFAULT_VARIABLE_QUERY: Partial<Query> = {
  queryType: 'resource-list',
  resourceType: 'server',
};

/**
 * These are options configured for each DataSource instance
 */
export interface DataSourceOptions extends DataSourceJsonData {
  debug: boolean;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface SecureJsonData {
  apiToken?: string;
}
