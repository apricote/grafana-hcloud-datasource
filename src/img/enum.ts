export const isValidOption = (enumType: Record<string, string>, value: string): boolean => {
  return Object.values(enumType).includes(value);
};
