# Cibim: pitch mətni (3:00)

## 1. Problem · 0:00–0:25
**Slayd:** "Bu ay nə qədər xərclədin?" + "21,7 mln kart · hər böyüyə orta hesabla 2,7"

> Zalda kimin birdən çox bank kartı var? *(əl qaldırmağı gözlə)*
> İndi sual: bu ay bütün kartlarınızda **ümumilikdə** nə qədər xərcləmisiniz? *(pauza)*
>
> Çoxumuz bilmirik. Azərbaycanda **21,7 milyon** bank kartı var. Bu, hər böyüyə orta hesabla 2,7 kart deməkdir. Maaş bir bankdadır, cashback kartı başqasında, taksit üçüncüsündə. Heç bir tətbiq onları bir yerdə göstərmir.

## 2. Həll · 0:25–0:50
**Slayd:** Cibim loqosu + "Banklar göstərir. Cibim fayda çıxarır."

> Bir müddət əvvəl Mərkəzi Bank yeni qayda qəbul etdi: **artıq sən öz bankına "mənim məlumatlarımı bu tətbiqə göstər" deyə icazə verə bilərsən.** Saytlara "Google ilə daxil ol" düyməsi ilə girmək kimi: bir toxunuş, bir icazə.
>
> Banklar artıq bundan istifadə edir, amma başqa bankdakı hesabını sənə uzun, qarışıq bir siyahı kimi göstərirlər.
>
> **Banklar sənə tranzaksiyalarını göstərir. Cibim isə onlardan sənə fayda çıxarır.**

## 3. Canlı demo · 0:50–2:05
**Ekranda tətbiq**

> Tanış olun, bu **Ayseldir**. Maaşı ABB kartına gəlir, gündəlik xərcləri Leobank kartındandır. Yəni zaldakıların yarısı kimi.
>
> Aysel bankını seçir. Bank tətbiqi açılır, **Face ID**, hazırdır. Parolu nə biz görürük, nə də başqası.
>
> Və ilk dəfə **bütün pulu bir ekranda**. Amma ən vacibi budur: *(Safe-to-Spend-ə işarə et)*
> **"Bu gün 22 ₼ xərcləyə bilərsən."**
> Cibim özü tapıb ki, maaş 15 noyabrda gəlir, ondan əvvəl kirayə, kredit və Netflix çıxacaq. Qalanını günlərə bölüb.
>
> Bankın Ayselə göstərdiyi budur: **"POS BRAVO HYPER 28 MAY BAKU AZ"**. Biri başa düşdü? *(pauza)*
> Cibim isə sadəcə yazır: **Bravo, ərzaq.** Burada da xəbərdarlıq var: **qaz pulu keçən aydan üç dəfə çox gəlib.**
>
> İndi bir az real həyat. Aysel ticarət mərkəzindədir, Zara-da gözəl bir pencək görür... *(komanda yoldaşına işarə: göndər)*
> 120 manat. Ödədi. *(refresh)*
> **"Bu alış həqiqətən lazım idi? Bu günün limiti 22 manat idi."** *(pauza)*
>
> Limit daxilində qaldığı hər gün Aysel **streak 🔥** qazanır, Duolingo kimi. Qənaət vərdişə çevrilir.

## 4. Niyə Cibim · 2:05–2:25
**Slayd:** rəqiblər, 2×2 (banklara qoşulur ↔ neytraldır)

> Banklar datanı göstərir, amma neytral deyillər, öz kartlarını və kreditlərini satırlar. Xərc izləyiciləri neytraldır, amma bankınıza qoşulmur, hər şeyi əl ilə yazırsınız. **Cibim hər ikisini edir.**
> Biz yalnız baxırıq: pul köçürə bilmirik, icazəni istənilən an geri götürmək olar.

## 5. Biznes modeli və bazar · 2:25–2:50
**Slayd:** "İstifadəçi üçün pulsuz" + 3 sətir + TAM/SAM/SOM (8 mln / 3,85 mln / 50 min)

> Cibim istifadəçi üçün həmişə pulsuzdur. Gəlir tərəfdaşlardandır: istifadəçinin öz xərclərinə görə şəxsi kart və depozit təklifləri göstəririk, bank yalnız gətirdiyimiz müştəriyə görə ödəyir.
> Bu model işləyir: **Credit Karma eyni modellə ildə 2,3 milyard dollar qazanır.**
> Azərbaycanda 3,85 milyon insan artıq rəqəmsal ödənişdən istifadə edir. İlk üç ildə hədəfimiz 50 min istifadəçidir.

## 6. Bağlanış · 2:50–3:00
**Slayd:** Cibim + "Növbəti addım: Mərkəzi Bankın sandbox-u, 500 istifadəçi"

> Növbəti addımımız Mərkəzi Bankın sandbox-unda 500 real istifadəçi ilə testdir.
> **Cibim: bütün kartların, bir cibdə.** Təşəkkür edirik.

---

## Deck: slayd siyahısı
1. **Problem:** sual + "21,7 mln kart / 2,7 kart"
2. **Həll:** loqo + əsas mesaj
3. **Demo:** canlı tətbiq (slayd yox, ekran)
4. **Rəqiblər:** 2×2. Banklar: qoşulur, neytral deyil. Zenmoney/CoinKeeper: neytral, qoşulmur. Cibim: hər ikisi
5. **Biznes modeli + bazar:** pulsuz · tərəfdaş təklifləri · B2B (kredit təşkilatları) + Credit Karma 2,3 mlrd $ + TAM/SAM/SOM
6. **Bağlanış:** sandbox, 500 istifadəçi + komanda (Ali, Hüseyn, Alimə)

## Qeydlər
- Pitch-dən 5 dəqiqə əvvəl: `curl -X POST https://cibim.farahmobilya.az/mock/reset`. Bankda təsdiq ilişsə, ehtiyat yol: `POST /api/v1/demo/connect-all`.
- Canlı tranzaksiyanı göndərən əmr `backend/README.md` faylının "On-stage demo" bölməsindədir. Kimin, hansı cihazdan göndərəcəyini əvvəlcədən razılaşın.
- 22 ₼ və 15 noyabr Aysel-in bugünkü datasındandır. Pitch günü tətbiqdə nə görünürsə, onu deyin.
- Vaxt çatmasa, kəsiləcək ilk yer: qaz xəbərdarlığı cümləsi. "Biri başa düşdü?" sualını və Zara anını saxlayın.
- Münsif "Open Banking" soruşsa: "Bəli, bunun adı Open Banking-dir, 2025-ci ilin oktyabrında Azərbaycanda başladı."
- Sual-cavab üçün hazır cavablar strategiya sənədinin §16.2 bölməsindədir. Dollar hesabı soruşulsa: Credit Karma-da tərəfdaşlar bir istifadəçiyə görə ildə ~16 $ ödəyir, Azərbaycan üçün 10 dəfə az götürürük.

## Mənbələr
- 21,7 mln kart (oktyabr 2025): [Trend.Az, Mərkəzi Bank məlumatı](https://www.trend.az/business/4126734.html)
- Əhali 10,26 mln: [APA, Statistika Komitəsi](https://en.apa.az/social/azerbaijans-population-reaches-1026-million-as-growth-continues-in-2025-486244); 0–14 yaş 21,8%: [Dünya Bankı](https://statisticsoftheworld.com/country/azerbaijan/population-under-15)
- 3,85 mln rəqəmsal ödəniş istifadəçisi: [Report.az, ADB](https://report.az/en/finance/adb-about-4m-people-in-azerbaijan-to-use-digital-payments-by-year-end)
- Credit Karma 2,3 mlrd $ (FY2025): [Intuit, Business Wire](https://www.businesswire.com/news/home/20250821275975/en/Intuit-Reports-Strong-Fourth-Quarter-and-Full-Year-Fiscal-2025-Results-Sets-Fiscal-2026-Guidance-With-Double-Digit-Revenue-Growth-and-Continued-Operating-Margin-Expansion)
