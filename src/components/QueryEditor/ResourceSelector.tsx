import React, { useCallback, useState } from 'react';
import { MultiCombobox, InlineField, ComboboxOption } from '@grafana/ui';

import type { DataSource } from '../../datasource';
import { notEmpty } from '../../util';
import { ResourceType } from '../../types';

interface ResourceSelectorFieldProps {
  datasource: DataSource;

  ids: number[];
  resourceType: ResourceType;

  onChange: (ids: number[]) => void;
}

export function ResourceSelectorField({ datasource, ids, resourceType, onChange }: ResourceSelectorFieldProps) {
  const [resources, setResources] = useState<Array<ComboboxOption<number>>>([]);

  const load = useCallback(
    async (_: string) => {
      let resources: Array<ComboboxOption<number>> = [];
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
      <MultiCombobox
        key={resourceType} // Force reloading options when the key changes
        options={load}
        value={values}
        onChange={(newValues: Array<ComboboxOption<number>>) => onChange(newValues.map((value) => value.value))}
        createCustomValue={false}
      ></MultiCombobox>
    </InlineField>
  );
}
