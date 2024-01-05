import { DataSourceInstanceSettings, CoreApp, SelectableValue, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { Query, DataSourceOptions, DEFAULT_QUERY, SelectBy } from './types';
import { VariableSupport } from './variables';

export class DataSource extends DataSourceWithBackend<Query, DataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<DataSourceOptions>) {
    super(instanceSettings);

    this.variables = new VariableSupport();
  }

  applyTemplateVariables(query: Query, scopedVars: ScopedVars): Query {
    const templateSrv = getTemplateSrv();

    if (query.labelSelectors) {
      query.labelSelectors = query.labelSelectors.map((selector) => templateSrv.replace(selector, scopedVars, 'json'));
    }

    if (query.selectBy === SelectBy.Name) {
      query.selectBy = SelectBy.ID;

      const replacedValue = templateSrv.replace(query.resourceIDsVariable, scopedVars, 'json');

      if (replacedValue !== '') {
        query.resourceIDs = (JSON.parse(replacedValue) as string[]).map((stringID) => parseInt(stringID, 10));
      } else {
        query.resourceIDs = [];
      }
    }

    return query;
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

  filterQuery(query: Query): boolean {
    if (query.selectBy === SelectBy.Name && query.resourceIDsVariable === '') {
      return false;
    }

    return true;
  }
}
