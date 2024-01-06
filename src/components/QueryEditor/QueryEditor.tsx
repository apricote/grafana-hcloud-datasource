import React, { useCallback } from 'react';
import { InlineFieldRow } from '@grafana/ui';
import type { QueryEditorProps } from '@grafana/data';

import { DataSource } from '../../datasource';
import {
  DataSourceOptions,
  LoadBalancerMetricsTypes,
  Query,
  QueryType,
  ResourceType,
  SelectBy,
  ServerMetricsTypes,
} from '../../types';
import { isValidOption } from '../../util';
import { OptionGroup } from '../OptionGroup';
import { LabelSelectorField } from './LabelSelector';
import { LegendFormatField } from './LegendFormat';
import { QueryTypeField } from './QueryType';
import { ResourceSelectorField } from './ResourceSelector';
import { MetricsTypeField } from './MetricsType';
import { ResourceTypeField } from './ResourceType';
import { SelectByField } from './SelectBy';
import { VariableSelectorField } from './VariableSelector';

type Props = QueryEditorProps<DataSource, Query, DataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const {
    queryType,
    resourceType,
    metricsType,
    selectBy,
    labelSelectors = [],
    resourceIDs = [],
    resourceIDsVariable = '',
    legendFormat = '',
  } = query;

  const onChangeRunQuery = useCallback(
    (newQuery: Query) => {
      onChange(newQuery);
      onRunQuery();
    },
    [onChange, onRunQuery]
  );

  const onResourceTypeChange = (resourceType: ResourceType) => {
    let metricsType = query.metricsType;

    // Make sure that the metrics type is valid for the new resource type
    switch (resourceType) {
      case ResourceType.Server: {
        if (!isValidOption(ServerMetricsTypes, metricsType)) {
          metricsType = ServerMetricsTypes.CPU;
        }
        break;
      }
      case ResourceType.LoadBalancer: {
        if (!isValidOption(LoadBalancerMetricsTypes, metricsType)) {
          metricsType = LoadBalancerMetricsTypes.Bandwidth;
        }
      }
    }
    onChange({ ...query, resourceType, metricsType, resourceIDs: [] });
    onRunQuery();
  };

  const collapsedInfoList = [`Query Type: ${queryType}`, `Legend: ${legendFormat !== '' ? legendFormat : 'Auto'}`];

  return (
    <>
      <InlineFieldRow>
        <ResourceTypeField resourceType={resourceType} onChange={onResourceTypeChange} />
        {queryType === QueryType.Metrics && (
          <MetricsTypeField
            metricsType={metricsType}
            resourceType={resourceType}
            onChange={(metricsType) => onChangeRunQuery({ ...query, metricsType })}
          />
        )}
      </InlineFieldRow>
      <InlineFieldRow>
        {queryType === QueryType.ResourceList && (
          <LabelSelectorField
            values={labelSelectors}
            onChange={(v) => onChangeRunQuery({ ...query, labelSelectors: v })}
          />
        )}
        {queryType === QueryType.Metrics && (
          <>
            <SelectByField selectBy={selectBy} onChange={(selectBy) => onChangeRunQuery({ ...query, selectBy })} />
            {selectBy === SelectBy.ID && (
              <ResourceSelectorField
                datasource={datasource}
                ids={resourceIDs}
                resourceType={resourceType}
                onChange={(resourceIDs) => onChangeRunQuery({ ...query, resourceIDs })}
              />
            )}
            {selectBy === SelectBy.Label && (
              <LabelSelectorField
                values={labelSelectors}
                onChange={(labelSelectors) => onChangeRunQuery({ ...query, labelSelectors })}
              />
            )}
            {selectBy === SelectBy.Name && (
              <VariableSelectorField
                variable={resourceIDsVariable}
                onChange={(resourceIDsVariable) => onChangeRunQuery({ ...query, resourceIDsVariable })}
              />
            )}
          </>
        )}
      </InlineFieldRow>

      <InlineFieldRow>
        <OptionGroup title={'Options'} collapsedInfo={collapsedInfoList}>
          <QueryTypeField queryType={queryType} onChange={(queryType) => onChangeRunQuery({ ...query, queryType })} />
          <LegendFormatField
            legendFormat={legendFormat}
            onChange={(legendFormat) => onChangeRunQuery({ ...query, legendFormat })}
          />
        </OptionGroup>
      </InlineFieldRow>
    </>
  );
}
