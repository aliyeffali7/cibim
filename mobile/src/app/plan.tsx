import { router } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Text, TextInput, View } from 'react-native';
import { Back, Body, Button, Caption, Card, categoryIconSpec, Chip, ErrorBox, H1, KV, Overline, Pill, Progress, Row, Screen } from '../components/ui';
import { api, Category, Money } from '../lib/api';
import { amount, money, shortDate } from '../lib/format';
import { colors, fonts } from '../lib/theme';

const CATS = [
  ['clothes', 'Clothes'],
  ['home', 'Home'],
  ['tech', 'Tech'],
  ['school', 'School'],
  ['health', 'Health'],
  ['assets', 'Assets'],
  ['other', 'Other'],
] as const;

type Check = {
  name: string;
  category: Category;
  price: Money;
  limit: Money | null;
  period: string | null;
  spentSoFar: Money | null;
  plannedOther: Money | null;
  projected: Money | null;
  pct: number | null;
  status: 'fits' | 'close' | 'exceeds' | 'no_limit';
  overage: Money | null;
  dataSince: string | null;
};

const PERIOD_WORD: Record<string, string> = { week: 'weekly', month: 'monthly', year: 'yearly' };
const SPENT_LABEL: Record<string, string> = { week: 'Spent this week', month: 'Spent this month', year: 'Spent this year' };

// 5.1 Plan a purchase (+ 8.6 no limit yet)
export default function Plan() {
  const [name, setName] = useState('');
  const [price, setPrice] = useState('');
  const [cat, setCat] = useState<string | null>(null);
  const [result, setResult] = useState<Check | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const seq = useRef(0);

  const priceMinor = Math.round(parseFloat(price.replace(',', '.').replace(/\s/g, '')) * 100);
  const ready = name.trim() && priceMinor > 0 && cat;

  useEffect(() => {
    setSaved(null);
    if (!ready) {
      setResult(null);
      return;
    }
    const mine = ++seq.current;
    const t = setTimeout(async () => {
      try {
        const r = await api<Check>('POST', '/purchase-plans/check', { name: name.trim(), price: { amountMinor: priceMinor, currency: 'AZN' }, categoryId: `cat-${cat}` });
        if (mine === seq.current) {
          setResult(r);
          setError(null);
        }
      } catch (e: any) {
        if (mine === seq.current) setError(e.message);
      }
    }, 350);
    return () => clearTimeout(t);
  }, [name, price, cat]);

  const save = async (status: 'planned' | 'dropped') => {
    setBusy(true);
    try {
      await api('POST', '/purchase-plans', { name: name.trim(), price: { amountMinor: priceMinor, currency: 'AZN' }, categoryId: `cat-${cat}`, status });
      setSaved(status === 'dropped' ? `Good call. ${amount(priceMinor)} ₼ stays in your pocket.` : 'Added to your plans.');
      setResult(null);
      setName('');
      setPrice('');
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  };

  const tone = result?.status === 'exceeds' ? 'negative' : result?.status === 'close' ? 'warning' : 'positive';
  const toneColor = { negative: colors.negative, warning: colors.warning, positive: colors.positive }[tone];
  const toneBg = { negative: colors.negativeSoft, warning: colors.warningSoft, positive: colors.positiveSoft }[tone];
  const r = result;
  const before = r?.limit && r.spentSoFar && r.plannedOther ? ((r.spentSoFar.amountMinor + r.plannedOther.amountMinor) / r.limit.amountMinor) * 100 : 0;
  const purchasePct = r?.limit ? (r.price.amountMinor / r.limit.amountMinor) * 100 : 0;
  const catName = CATS.find(([k]) => k === cat)?.[1] ?? '';

  return (
    <Screen>
      <Back label="Home" />
      <View style={{ gap: 6 }}>
        <H1 style={{ fontSize: 26 }}>Plan a purchase</H1>
        <Body style={{ color: colors.textSecondary }}>Check it against your own limit before the money leaves.</Body>
      </View>

      <View style={{ gap: 8 }}>
        <Overline>Item</Overline>
        <TextInput value={name} onChangeText={setName} placeholder="Washing machine" placeholderTextColor={colors.textMuted} style={inputStyle} />
      </View>
      <View style={{ gap: 8 }}>
        <Overline>Price</Overline>
        <View>
          <TextInput value={price} onChangeText={setPrice} placeholder="1 200" keyboardType="decimal-pad" placeholderTextColor={colors.textMuted} style={inputStyle} />
          <Text style={{ position: 'absolute', right: 16, top: 15, color: colors.textSecondary, fontSize: 16 }}>₼</Text>
        </View>
      </View>
      <View style={{ gap: 8 }}>
        <Overline>Category</Overline>
        <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
          {CATS.map(([k, label]) => (
            <Chip key={k} label={label} icon={categoryIconSpec(k)} active={cat === k} onPress={() => setCat(k)} />
          ))}
        </View>
      </View>

      {error ? <ErrorBox message={error} /> : null}
      {saved ? (
        <Card style={{ backgroundColor: colors.positiveSoft }}>
          <Body style={{ color: colors.positive }}>{saved}</Body>
        </Card>
      ) : null}

      {r && r.status === 'no_limit' ? (
        <Card style={{ gap: 10 }}>
          <Pill label="No limit yet" />
          <Text style={{ fontFamily: fonts.heading, fontSize: 18, color: colors.text }}>Set a {catName} limit first</Text>
          <Body style={{ color: colors.textSecondary }}>Cibim compares purchases with your own limit. Set one and check again.</Body>
          <Button label={`Set ${catName} limit`} onPress={() => router.push({ pathname: '/budget-edit', params: { category: cat! } })} />
        </Card>
      ) : null}

      {r && r.status !== 'no_limit' && r.limit ? (
        <View style={{ backgroundColor: toneBg, borderRadius: 18, padding: 18, gap: 12 }}>
          <Row style={{ justifyContent: 'space-between' }}>
            <Pill solid tone={tone} label={r.status === 'exceeds' ? 'Exceeds limit' : r.status === 'close' ? 'Close to limit' : 'Fits your limit'} />
            <Text style={{ fontFamily: fonts.heading, fontSize: 22, color: toneColor }}>{r.pct}%</Text>
          </Row>
          <Text style={{ fontFamily: fonts.heading, fontSize: 19, color: colors.text }}>
            {r.status === 'exceeds' ? `Over your ${catName} limit by ${amount(r.overage!.amountMinor)} ₼` : r.status === 'close' ? `Close to your ${catName} limit` : `Fits your ${catName} limit`}
          </Text>
          <Progress pct={before} extraPct={purchasePct} extraColor={r.status === 'fits' ? colors.positive : toneColor} />
          <Caption style={{ fontSize: 14 }}>
            Projected {money(r.projected)} of your {money(r.limit)} {PERIOD_WORD[r.period ?? 'month']} limit.
          </Caption>
          <Card style={{ paddingVertical: 8 }}>
            <KV label={SPENT_LABEL[r.period ?? 'month']} value={money(r.spentSoFar)} />
            {r.plannedOther && r.plannedOther.amountMinor > 0 ? <KV label="Other plans" value={money(r.plannedOther)} /> : null}
            <KV label="This purchase" value={money(r.price)} />
            <KV label={`${PERIOD_WORD[r.period ?? 'month'][0].toUpperCase()}${PERIOD_WORD[r.period ?? 'month'].slice(1)} limit`} value={money(r.limit)} />
          </Card>
          {r.dataSince ? (
            <Caption>
              Based on data since {shortDate(r.dataSince)}.{' '}
              <Text onPress={() => router.push({ pathname: '/budget-edit', params: { category: cat! } })} style={{ color: colors.brand, fontFamily: fonts.medium }}>
                Add earlier spending
              </Text>
            </Caption>
          ) : null}
          {r.status === 'fits' ? (
            <Button label="Plan it" loading={busy} onPress={() => save('planned')} />
          ) : (
            <Row style={{ gap: 10 }}>
              <View style={{ flex: 1 }}>
                <Button label="Don't buy" loading={busy} onPress={() => save('dropped')} />
              </View>
              <View style={{ flex: 1 }}>
                <Button label="Skip" variant="secondary" onPress={() => setResult(null)} />
              </View>
            </Row>
          )}
        </View>
      ) : null}
    </Screen>
  );
}

const inputStyle = {
  backgroundColor: '#fff',
  borderRadius: 14,
  borderWidth: 1,
  borderColor: colors.border,
  paddingHorizontal: 16,
  height: 52,
  fontSize: 16,
  fontFamily: fonts.body,
  color: colors.text,
} as const;
