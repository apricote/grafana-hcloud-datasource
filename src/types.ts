import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export enum QueryType {
  ResourceList = 'resource-list',
  Metrics = 'metrics',
}

export enum ResourceType {
  Server = 'server',
  LoadBalancer = 'load-balancer',
}

export enum ServerMetricsTypes {
  CPU = 'cpu',
  DiskBandwidth = 'disk-bandwidth',
  DiskIOPS = 'disk-iops',
  NetworkBandwidth = 'network-bandwidth',
  NetworkPPS = 'network-pps',
}

export enum LoadBalancerMetricsTypes {
  OpenConnections = 'open-connections',
  ConnectionsPerSecond = 'connections-per-second',
  RequestsPerSecond = 'requests-per-second',
  Bandwidth = 'bandwidth',
}

export type MetricsType = ServerMetricsTypes | LoadBalancerMetricsTypes;

export enum SelectBy {
  Label = 'label',
  ID = 'id',
  Name = 'name',
}

export interface Query extends DataQuery {
  queryType: QueryType;
  resourceType: ResourceType;
  metricsType: MetricsType;

  selectBy: SelectBy;
  labelSelectors: string[];
  resourceIDs: number[];
  resourceIDsVariable: string;

  legendFormat: string;
}

export const DEFAULT_QUERY: Partial<Query> = {
  queryType: QueryType.Metrics,
  resourceType: ResourceType.Server,
  metricsType: ServerMetricsTypes.CPU,
  selectBy: SelectBy.Label,
  labelSelectors: [],
  resourceIDs: [],
  resourceIDsVariable: '',
  legendFormat: '',
};

export const DEFAULT_VARIABLE_QUERY: Partial<Query> = {
  queryType: QueryType.ResourceList,
  resourceType: ResourceType.Server,

  labelSelectors: [],
  resourceIDs: [],
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
