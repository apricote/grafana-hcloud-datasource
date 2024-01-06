import { InlineField, LinkButton, TagsInput } from '@grafana/ui';
import React from 'react';

interface LabelSelectorFieldProps {
  values: string[];
  onChange: (values: string[]) => void;
}
export function LabelSelectorField({ values, onChange }: LabelSelectorFieldProps) {
  return (
    <InlineField
      label={'Label Selectors'}
      tooltip={
        <LinkButton
          href="https://docs.hetzner.cloud/#label-selector"
          size="sm"
          variant="secondary"
          icon="external-link-alt"
          target="_blank"
        >
          Docs
        </LinkButton>
      }
      interactive={true} // So user can click the link
    >
      <TagsInput tags={values} onChange={onChange} placeholder={'Selectors (enter key to add)'} />
    </InlineField>
  );
}
