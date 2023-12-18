import { DataSourceQueryType, DataSourceVariableSupport } from '@grafana/data';
import { DataSource } from './datasource';
import { DEFAULT_VARIABLE_QUERY } from './types';

export class VariableSupport extends DataSourceVariableSupport<DataSource> {
  getDefaultQuery(): Partial<DataSourceQueryType<DataSource>> {
    return DEFAULT_VARIABLE_QUERY;
  }
}
