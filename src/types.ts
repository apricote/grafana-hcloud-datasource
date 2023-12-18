import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export interface Query extends DataQuery {
  queryType: 'resource-list' | 'metrics';
  resourceType: 'server' | 'load-balancer';
  resourceIds: number[];
}

export const DEFAULT_QUERY: Partial<Query> = {
  queryType: 'metrics',
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
