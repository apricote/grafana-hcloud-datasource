import React, { useCallback, useState } from 'react';
import { AsyncMultiSelect, InlineField } from '@grafana/ui';
import { ResourceType } from '../../types';
import { SelectableValue } from '@grafana/data';
import type { DataSource } from '../../datasource';
import { notEmpty } from '../../util';

interface ResourceSelectorFieldProps {
  datasource: DataSource;

  ids: number[];
  resourceType: ResourceType;

  onChange: (ids: number[]) => void;
}

export function ResourceSelectorField({ datasource, ids, resourceType, onChange }: ResourceSelectorFieldProps) {
  const [resources, setResources] = useState<Array<SelectableValue<number>>>([]);

  const load = useCallback(
    async (_: string) => {
      let resources: Array<SelectableValue<number>> = [];
      switch (resourceType) {
        case ResourceType.Server: {
          resources = await datasource.getServers();
          break;
        }
        case ResourceType.LoadBalancer: {
          resources = await datasource.getLoadBalancers();
          break;
        }
      }

      setResources(resources);
      return resources;
    },
    [datasource, resourceType]
  );

  const values = ids.map((id) => resources.find((r) => r.value === id)).filter(notEmpty);

  return (
    <InlineField label={resourceType === ResourceType.Server ? 'Servers' : 'Load Balancers'}>
      <AsyncMultiSelect
        key={resourceType} // Force reloading options when the key changes
        loadOptions={load}
        value={values}
        onChange={(newValues: Array<SelectableValue<number>>) => onChange(newValues.map((value) => value.value!))}
        defaultOptions
        isSearchable={false} // Currently not implemented in loadResources + API methods
      ></AsyncMultiSelect>
    </InlineField>
  );
}
