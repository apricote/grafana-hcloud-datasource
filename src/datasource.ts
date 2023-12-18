import { DataSourceInstanceSettings, CoreApp } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';

import { Query, DataSourceOptions, DEFAULT_QUERY } from './types';
import { VariableSupport } from './variables';

export class DataSource extends DataSourceWithBackend<Query, DataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<DataSourceOptions>) {
    super(instanceSettings);

    this.variables = new VariableSupport();
  }

  getDefaultQuery(_: CoreApp): Partial<Query> {
    return DEFAULT_QUERY;
  }
}
