import React, { useCallback, useState } from 'react';
import { InlineField, Input } from '@grafana/ui';

interface VariableSelectorFieldProps {
  variable: string;
  onChange: (variable: string) => void;
}

export const VariableSelectorField = ({ variable, onChange }: VariableSelectorFieldProps) => {
  const [error, setError] = useState<string | null>(null);

  const onChangeWithValidation = useCallback(
    (e: React.FormEvent<HTMLInputElement>) => {
      if (!e.currentTarget.value.startsWith('$')) {
        setError('Variable name must start with $');
      } else {
        setError(null);
      }

      onChange(e.currentTarget.value);
    },
    [onChange]
  );

  return (
    <InlineField label={'Variable Name'} tooltip={'Make sure to prefix with $'} invalid={error != null} error={error}>
      <Input value={variable} placeholder={'$variableName'} onChange={onChangeWithValidation}></Input>
    </InlineField>
  );
};
