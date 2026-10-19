import { Text, View } from 'react-native';
import { colors, fonts } from '../lib/theme';

/** Wordmark: "cibim" with a pocket-shaped mark over the c. */
export function Logo({ color = colors.brand, size = 40 }: { color?: string; size?: number }) {
  return (
    <View style={{ flexDirection: 'row', alignItems: 'flex-end' }}>
      <View style={{ marginRight: size * 0.04, marginBottom: size * 0.14 }}>
        <View
          style={{
            width: size * 0.62,
            height: size * 0.72,
            borderRadius: size * 0.16,
            borderTopLeftRadius: size * 0.36,
            borderWidth: size * 0.13,
            borderColor: color,
            borderRightWidth: 0,
          }}
        />
      </View>
      <Text style={{ color, fontFamily: fonts.headingXB, fontSize: size * 1.1, lineHeight: size * 1.2, letterSpacing: -1 }}>ibim</Text>
    </View>
  );
}
