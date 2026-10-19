import { router } from 'expo-router';
import { useEffect } from 'react';
import { Loading, Screen } from '../components/ui';

// Where the bank's approval page sends the user back (deep link). The connect
// screen polls the result itself, so this just returns to where the user was.
export default function Connected() {
  useEffect(() => {
    const t = setTimeout(() => (router.canGoBack() ? router.back() : router.replace('/home')), 300);
    return () => clearTimeout(t);
  }, []);
  return (
    <Screen>
      <Loading />
    </Screen>
  );
}
