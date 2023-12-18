import { DataSourceInstanceSettings, CoreApp } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';

import { Query, DataSourceOptions, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<Query, DataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<DataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_: CoreApp): Partial<Query> {
    return DEFAULT_QUERY;
  }
}
