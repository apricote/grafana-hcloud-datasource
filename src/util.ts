export const isValidOption = (enumType: Record<string, string>, value: string): boolean => {
  return Object.values(enumType).includes(value);
};

export const notEmpty = <TValue>(value: TValue | null | undefined): value is TValue =>
  value !== null && value !== undefined;
