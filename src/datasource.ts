import { DataSourceInstanceSettings, CoreApp, SelectableValue } from '@grafana/data';
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

  async getServers(): Promise<Array<SelectableValue<number>>> {
    return this.getResource('/servers');
  }

  async getLoadBalancers(): Promise<Array<SelectableValue<number>>> {
    return this.getResource('/load-balancers');
  }
}
