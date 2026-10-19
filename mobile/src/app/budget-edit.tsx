import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { Text, TextInput, View } from 'react-native';
import { Back, Body, Button, Caption, categoryIconSpec, Chip, ErrorBox, H1, Overline, Row, Screen } from '../components/ui';
import { api, Budget, Category } from '../lib/api';
import { shortDate } from '../lib/format';
import { useApi } from '../lib/session';
import { colors, fonts } from '../lib/theme';

const HINT: Record<string, string> = {
  assets: 'Big one-off buys: furniture, appliances, electronics.',
  clothes: 'Clothes, shoes and accessories.',
  tech: 'Phones, laptops and gadgets.',
  home: 'Things for the home.',
  school: 'School supplies, uniforms and books.',
  health: 'Pharmacy, doctors and care.',
};
const PICK = ['clothes', 'home', 'tech', 'school', 'health', 'assets', 'other', 'dining', 'groceries', 'transport', 'shopping'];

// 5.3 Edit limit
export default function BudgetEdit() {
  const params = useLocalSearchParams<{ id?: string; category?: string }>();
  const budgets = useApi<Budget[]>('/budgets');
  const cats = useApi<Category[]>('/categories');
  const existing = budgets.data?.find((b) => b.id === params.id) ??
    budgets.data?.find((b) => b.basis === 'user' && params.category && b.category?.key === params.category);
  const [category, setCategory] = useState<string>(params.category ?? '');
  const [period, setPeriod] = useState<'week' | 'month' | 'year'>(params.category && ['assets', 'tech', 'home', 'school'].includes(params.category) ? 'year' : 'month');
  const [limit, setLimit] = useState('');
  const [spent, setSpent] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (existing) {
      setPeriod(existing.period);
      setLimit((existing.limit.amountMinor / 100).toFixed(2));
      if (existing.alreadySpent.amountMinor) setSpent((existing.alreadySpent.amountMinor / 100).toFixed(2));
    }
  }, [existing?.id]);

  const toMinor = (s: string) => Math.round(parseFloat(s.replace(',', '.').replace(/\s/g, '') || '0') * 100);
  const catName = category ? cats.data?.find((c) => c.key === category)?.name ?? category : 'All spending';
  const dataSince = budgets.data?.[0]?.dataSince;

  const save = async () => {
    if (toMinor(limit) <= 0) return setError('Enter a limit.');
    setSaving(true);
    try {
      const body = {
        categoryId: category ? `cat-${category}` : '',
        period,
        limit: { amountMinor: toMinor(limit), currency: 'AZN' },
        alreadySpent: { amountMinor: toMinor(spent), currency: 'AZN' },
      };
      if (existing) await api('PATCH', `/budgets/${existing.id}`, body);
      else await api('POST', '/budgets', body);
      router.back();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setSaving(false);
    }
  };

  const periodWord = period === 'week' ? 'week' : period === 'year' ? 'year' : 'month';
  return (
    <Screen footer={<Button label="Save limit" onPress={save} loading={saving} />}>
      <Back />
      <View style={{ gap: 6 }}>
        <H1 style={{ fontSize: 26 }}>{catName} limit</H1>
        <Body style={{ color: colors.textSecondary }}>{HINT[category] ?? 'Cibim warns you at 80% of this limit.'}</Body>
      </View>
      {!params.category && !existing ? (
        <View style={{ gap: 8 }}>
          <Overline>Category</Overline>
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
            {PICK.map((k) => (
              <Chip key={k} label={cats.data?.find((c) => c.key === k)?.name ?? k} icon={categoryIconSpec(k)} active={category === k} onPress={() => setCategory(k)} />
            ))}
          </View>
        </View>
      ) : null}
      <View style={{ gap: 8 }}>
        <Overline>Period</Overline>
        <Row style={{ gap: 8 }}>
          {(['week', 'month', 'year'] as const).map((p) => (
            <Chip key={p} label={p === 'week' ? 'Weekly' : p === 'month' ? 'Monthly' : 'Yearly'} active={period === p} onPress={() => setPeriod(p)} />
          ))}
        </Row>
      </View>
      <View style={{ gap: 8 }}>
        <Overline>Limit</Overline>
        <Money value={limit} onChange={setLimit} placeholder="5 000.00" />
      </View>
      <View style={{ gap: 8 }}>
        <Overline>Already spent this {periodWord}</Overline>
        <Money value={spent} onChange={setSpent} placeholder="0.00" />
        <Caption>
          Spending{dataSince ? ` before ${shortDate(dataSince)}` : ''} that your banks don't share. Cibim adds it to what it sees.
        </Caption>
      </View>
      {error ? <ErrorBox message={error} /> : null}
    </Screen>
  );
}

function Money({ value, onChange, placeholder }: { value: string; onChange: (v: string) => void; placeholder: string }) {
  return (
    <View>
      <TextInput
        value={value}
        onChangeText={onChange}
        placeholder={placeholder}
        keyboardType="decimal-pad"
        placeholderTextColor={colors.textMuted}
        style={{ backgroundColor: '#fff', borderRadius: 14, borderWidth: 1, borderColor: colors.border, paddingHorizontal: 16, height: 52, fontSize: 16, fontFamily: fonts.body, color: colors.text }}
      />
      <Text style={{ position: 'absolute', right: 16, top: 15, color: colors.textSecondary, fontSize: 16 }}>₼</Text>
    </View>
  );
}
