import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart' hide TextDirection;
import 'package:tiktok_clone/features/analytics/data/datasources/analytics_remote_datasource.dart';
import 'package:tiktok_clone/features/analytics/presentation/providers/analytics_provider.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Design tokens
// ─────────────────────────────────────────────────────────────────────────────

const _kBg = Color(0xFF0A0A0A);
const _kSurface = Color(0xFF161616);
const _kSurfaceElevated = Color(0xFF1F1F1F);
const _kTeal = Color(0xFF20D9A0);
const _kRed = Color(0xFFFF2D55);
const _kOrange = Color(0xFFFF6B35);
const _kBlue = Color(0xFF3D8EFF);
const _kTextPrimary = Color(0xFFFFFFFF);
const _kTextSecondary = Color(0xFF8E8E93);
const _kGrid = Color(0xFF2A2A2A);

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

String _fmt(num n) {
  if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
  if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
  return n.toStringAsFixed(0);
}

String _shortDate(String isoDate) {
  try {
    final dt = DateTime.parse(isoDate);
    return DateFormat('M/d').format(dt);
  } catch (_) {
    return isoDate.length >= 5 ? isoDate.substring(5) : isoDate;
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Analytics Screen
// ─────────────────────────────────────────────────────────────────────────────

class AnalyticsScreen extends ConsumerStatefulWidget {
  const AnalyticsScreen({super.key});

  @override
  ConsumerState<AnalyticsScreen> createState() => _AnalyticsScreenState();
}

class _AnalyticsScreenState extends ConsumerState<AnalyticsScreen> {
  String _period = '7d';

  @override
  Widget build(BuildContext context) {
    final asyncData = ref.watch(analyticsProvider);

    return Scaffold(
      backgroundColor: _kBg,
      appBar: AppBar(
        backgroundColor: _kBg,
        elevation: 0,
        centerTitle: true,
        title: const Text(
          'Analytics',
          style: TextStyle(
            color: _kTextPrimary,
            fontSize: 17,
            fontWeight: FontWeight.w600,
            letterSpacing: -0.3,
          ),
        ),
        iconTheme: const IconThemeData(color: _kTextPrimary),
        surfaceTintColor: Colors.transparent,
      ),
      body: Column(
        children: [
          _PeriodPicker(
            selected: _period,
            onChanged: (p) {
              setState(() => _period = p);
              ref.read(analyticsProvider.notifier).setPeriod(p);
            },
          ),
          Expanded(
            child: asyncData.when(
              loading: () => const Center(
                child: CircularProgressIndicator(
                  color: _kTeal,
                  strokeWidth: 2,
                ),
              ),
              error: (err, _) => Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.wifi_off_rounded,
                        color: _kTextSecondary, size: 40),
                    const SizedBox(height: 12),
                    Text(
                      'Could not load analytics',
                      style: TextStyle(color: _kTextSecondary, fontSize: 14),
                    ),
                    const SizedBox(height: 16),
                    TextButton(
                      onPressed: () =>
                          ref.read(analyticsProvider.notifier).setPeriod(_period),
                      child: const Text('Retry',
                          style: TextStyle(color: _kTeal)),
                    ),
                  ],
                ),
              ),
              data: (data) => _AnalyticsBody(data: data, period: _period),
            ),
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Period picker
// ─────────────────────────────────────────────────────────────────────────────

class _PeriodPicker extends StatelessWidget {
  const _PeriodPicker({required this.selected, required this.onChanged});
  final String selected;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) {
    const periods = [('7D', '7d'), ('28D', '28d'), ('90D', '90d')];
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Row(
        children: periods.map((p) {
          final isActive = selected == p.$2;
          return Padding(
            padding: const EdgeInsets.only(right: 8),
            child: GestureDetector(
              onTap: () => onChanged(p.$2),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 180),
                padding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 7),
                decoration: BoxDecoration(
                  color: isActive ? _kTeal : _kSurface,
                  borderRadius: BorderRadius.circular(20),
                  border: isActive
                      ? null
                      : Border.all(color: _kGrid, width: 1),
                ),
                child: Text(
                  p.$1,
                  style: TextStyle(
                    color: isActive ? Colors.black : _kTextSecondary,
                    fontWeight: FontWeight.w600,
                    fontSize: 13,
                    letterSpacing: 0.2,
                  ),
                ),
              ),
            ),
          );
        }).toList(),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Body — all chart sections
// ─────────────────────────────────────────────────────────────────────────────

class _AnalyticsBody extends StatelessWidget {
  const _AnalyticsBody({required this.data, required this.period});
  final AnalyticsData data;
  final String period;

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 40),
      children: [
        _OverviewCards(data: data),
        const SizedBox(height: 24),
        _SectionHeader(title: 'Views'),
        _LineChartCard(
          points: data.viewsData,
          lineColor: _kTeal,
          label: 'views',
        ),
        const SizedBox(height: 24),
        _SectionHeader(title: 'Top Videos'),
        _TopVideosSection(videos: data.topVideos),
        const SizedBox(height: 24),
        _SectionHeader(title: 'Traffic Sources'),
        _TrafficSourcesCard(sources: data.trafficSources),
        const SizedBox(height: 24),
        _SectionHeader(title: 'Follower Growth'),
        _BarChartCard(points: data.followerGrowth),
        const SizedBox(height: 24),
        _SectionHeader(title: 'Revenue'),
        _RevenueSection(
          points: data.revenueByDay,
          total: data.totalRevenue,
        ),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Overview metric cards
// ─────────────────────────────────────────────────────────────────────────────

class _OverviewCards extends StatelessWidget {
  const _OverviewCards({required this.data});
  final AnalyticsData data;

  @override
  Widget build(BuildContext context) {
    final items = [
      ('Total Views', _fmt(data.totalViews), _kTeal),
      ('New Followers', _fmt(data.newFollowers), _kBlue),
      ('Engagement', '${data.engagementRate.toStringAsFixed(1)}%', _kOrange),
      ('Revenue', '\$${data.totalRevenue.toStringAsFixed(2)}', _kRed),
    ];
    return GridView.count(
      crossAxisCount: 2,
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      mainAxisSpacing: 10,
      crossAxisSpacing: 10,
      childAspectRatio: 1.6,
      children: items.map((item) {
        return Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            color: _kSurface,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: _kGrid, width: 1),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                item.$1,
                style: const TextStyle(
                  color: _kTextSecondary,
                  fontSize: 11,
                  fontWeight: FontWeight.w500,
                  letterSpacing: 0.5,
                ),
              ),
              Text(
                item.$2,
                style: TextStyle(
                  color: item.$3,
                  fontSize: 26,
                  fontWeight: FontWeight.w700,
                  letterSpacing: -0.5,
                ),
              ),
            ],
          ),
        );
      }).toList(),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Section header
// ─────────────────────────────────────────────────────────────────────────────

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.title});
  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Text(
        title,
        style: const TextStyle(
          color: _kTextPrimary,
          fontSize: 16,
          fontWeight: FontWeight.w600,
          letterSpacing: -0.2,
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Line chart (custom painter — no fl_chart dependency needed)
// ─────────────────────────────────────────────────────────────────────────────

class _LineChartCard extends StatefulWidget {
  const _LineChartCard({
    required this.points,
    required this.lineColor,
    required this.label,
  });
  final List<DateCountPoint> points;
  final Color lineColor;
  final String label;

  @override
  State<_LineChartCard> createState() => _LineChartCardState();
}

class _LineChartCardState extends State<_LineChartCard> {
  int? _touchedIndex;

  @override
  Widget build(BuildContext context) {
    if (widget.points.isEmpty) {
      return _emptyChart();
    }
    return Container(
      height: 180,
      padding: const EdgeInsets.fromLTRB(12, 16, 12, 10),
      decoration: BoxDecoration(
        color: _kSurface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: _kGrid, width: 1),
      ),
      child: Column(
        children: [
          Expanded(
            child: GestureDetector(
              onTapDown: (d) {
                final box = context.findRenderObject() as RenderBox;
                final local = box.globalToLocal(d.globalPosition);
                final idx = (local.dx /
                        (box.size.width / widget.points.length))
                    .floor()
                    .clamp(0, widget.points.length - 1);
                setState(() => _touchedIndex = idx);
              },
              onTapUp: (_) =>
                  Future.delayed(const Duration(seconds: 2),
                      () => setState(() => _touchedIndex = null)),
              child: CustomPaint(
                painter: _LineChartPainter(
                  points: widget.points.map((p) => p.count).toList(),
                  lineColor: widget.lineColor,
                  gridColor: _kGrid,
                  touchedIndex: _touchedIndex,
                ),
                size: Size.infinite,
              ),
            ),
          ),
          const SizedBox(height: 6),
          _DateLabels(points: widget.points),
        ],
      ),
    );
  }

  Widget _emptyChart() => Container(
        height: 180,
        decoration: BoxDecoration(
          color: _kSurface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: _kGrid, width: 1),
        ),
        child: const Center(
          child: Text('No data', style: TextStyle(color: _kTextSecondary)),
        ),
      );
}

class _LineChartPainter extends CustomPainter {
  _LineChartPainter({
    required this.points,
    required this.lineColor,
    required this.gridColor,
    this.touchedIndex,
  });

  final List<double> points;
  final Color lineColor;
  final Color gridColor;
  final int? touchedIndex;

  @override
  void paint(Canvas canvas, Size size) {
    if (points.isEmpty) return;

    final maxVal = points.reduce(math.max);
    final minVal = points.reduce(math.min);
    final range = (maxVal - minVal).clamp(1.0, double.infinity);

    double xFor(int i) => size.width * i / (points.length - 1).clamp(1, 9999);
    double yFor(double v) =>
        size.height - (size.height * 0.1) -
        ((v - minVal) / range) * (size.height * 0.8);

    // Grid lines
    final gridPaint = Paint()
      ..color = gridColor
      ..strokeWidth = 0.5;
    for (var i = 0; i < 4; i++) {
      final y = size.height * 0.1 + (size.height * 0.8) * (i / 3);
      canvas.drawLine(Offset(0, y), Offset(size.width, y), gridPaint);
    }

    // Fill gradient
    final path = Path();
    path.moveTo(xFor(0), yFor(points[0]));
    for (var i = 1; i < points.length; i++) {
      path.lineTo(xFor(i), yFor(points[i]));
    }
    path.lineTo(xFor(points.length - 1), size.height);
    path.lineTo(0, size.height);
    path.close();

    canvas.drawPath(
      path,
      Paint()
        ..shader = LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: [
            lineColor.withOpacity(0.25),
            lineColor.withOpacity(0.0),
          ],
        ).createShader(Rect.fromLTWH(0, 0, size.width, size.height)),
    );

    // Line
    final linePaint = Paint()
      ..color = lineColor
      ..strokeWidth = 2
      ..strokeCap = StrokeCap.round
      ..strokeJoin = StrokeJoin.round
      ..style = PaintingStyle.stroke;

    final linePath = Path();
    linePath.moveTo(xFor(0), yFor(points[0]));
    for (var i = 1; i < points.length; i++) {
      linePath.lineTo(xFor(i), yFor(points[i]));
    }
    canvas.drawPath(linePath, linePaint);

    // Touched dot + tooltip
    if (touchedIndex != null && touchedIndex! < points.length) {
      final tx = xFor(touchedIndex!);
      final ty = yFor(points[touchedIndex!]);

      canvas.drawCircle(
        Offset(tx, ty),
        5,
        Paint()..color = lineColor,
      );

      // Vertical dashed line
      final dashPaint = Paint()
        ..color = lineColor.withOpacity(0.4)
        ..strokeWidth = 1;
      for (var dy = 0.0; dy < size.height; dy += 8) {
        canvas.drawLine(Offset(tx, dy), Offset(tx, dy + 4), dashPaint);
      }

      // Tooltip bubble
      final text = _fmt(points[touchedIndex!]);
      final tp = TextPainter(
        text: TextSpan(
          text: text,
          style: const TextStyle(
            color: Colors.black,
            fontSize: 11,
            fontWeight: FontWeight.w700,
          ),
        ),
        textDirection: TextDirection.ltr,
      )..layout();

      const pad = 6.0;
      final bubbleRect = RRect.fromRectAndRadius(
        Rect.fromCenter(
          center: Offset(tx, ty - 20),
          width: tp.width + pad * 2,
          height: tp.height + pad,
        ),
        const Radius.circular(4),
      );
      canvas.drawRRect(bubbleRect, Paint()..color = lineColor);
      tp.paint(
        canvas,
        Offset(tx - tp.width / 2, ty - 20 - tp.height / 2),
      );
    }
  }

  @override
  bool shouldRepaint(_LineChartPainter old) =>
      old.touchedIndex != touchedIndex ||
      old.points != points;
}

class _DateLabels extends StatelessWidget {
  const _DateLabels({required this.points});
  final List<DateCountPoint> points;

  @override
  Widget build(BuildContext context) {
    if (points.length < 2) return const SizedBox.shrink();
    // Show first, middle, last
    final indices = [0, points.length ~/ 2, points.length - 1];
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: indices.map((i) {
        return Text(
          _shortDate(points[i].date),
          style: const TextStyle(color: _kTextSecondary, fontSize: 10),
        );
      }).toList(),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Bar chart (follower growth)
// ─────────────────────────────────────────────────────────────────────────────

class _BarChartCard extends StatelessWidget {
  const _BarChartCard({required this.points});
  final List<DateCountPoint> points;

  @override
  Widget build(BuildContext context) {
    if (points.isEmpty) {
      return Container(
        height: 160,
        decoration: BoxDecoration(
          color: _kSurface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: _kGrid, width: 1),
        ),
        child: const Center(
          child: Text('No data', style: TextStyle(color: _kTextSecondary)),
        ),
      );
    }
    return Container(
      height: 160,
      padding: const EdgeInsets.fromLTRB(12, 16, 12, 10),
      decoration: BoxDecoration(
        color: _kSurface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: _kGrid, width: 1),
      ),
      child: Column(
        children: [
          Expanded(
            child: CustomPaint(
              painter: _BarChartPainter(
                points: points.map((p) => p.count).toList(),
                positiveColor: _kTeal,
                negativeColor: _kRed,
                gridColor: _kGrid,
              ),
              size: Size.infinite,
            ),
          ),
          const SizedBox(height: 6),
          _DateLabels(points: points),
        ],
      ),
    );
  }
}

class _BarChartPainter extends CustomPainter {
  _BarChartPainter({
    required this.points,
    required this.positiveColor,
    required this.negativeColor,
    required this.gridColor,
  });

  final List<double> points;
  final Color positiveColor;
  final Color negativeColor;
  final Color gridColor;

  @override
  void paint(Canvas canvas, Size size) {
    if (points.isEmpty) return;
    final maxAbs = points.map((p) => p.abs()).reduce(math.max).clamp(1.0, double.infinity);
    final midY = size.height / 2;
    final barW = (size.width / points.length) * 0.6;
    final gap = size.width / points.length;

    // Baseline
    canvas.drawLine(
      Offset(0, midY),
      Offset(size.width, midY),
      Paint()
        ..color = _kGrid
        ..strokeWidth = 0.5,
    );

    for (var i = 0; i < points.length; i++) {
      final val = points[i];
      final barH = (val.abs() / maxAbs) * (size.height * 0.45);
      final x = gap * i + gap / 2 - barW / 2;
      final color = val >= 0 ? positiveColor : negativeColor;
      final top = val >= 0 ? midY - barH : midY;

      canvas.drawRRect(
        RRect.fromRectAndRadius(
          Rect.fromLTWH(x, top, barW, barH),
          const Radius.circular(2),
        ),
        Paint()..color = color.withOpacity(0.85),
      );
    }
  }

  @override
  bool shouldRepaint(_BarChartPainter old) => old.points != points;
}

// ─────────────────────────────────────────────────────────────────────────────
// Top videos section
// ─────────────────────────────────────────────────────────────────────────────

class _TopVideosSection extends StatelessWidget {
  const _TopVideosSection({required this.videos});
  final List<TopVideo> videos;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: _kSurface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: _kGrid, width: 1),
      ),
      child: ListView.separated(
        shrinkWrap: true,
        physics: const NeverScrollableScrollPhysics(),
        itemCount: videos.take(5).length,
        separatorBuilder: (_, __) =>
            const Divider(color: _kGrid, height: 1, thickness: 1),
        itemBuilder: (context, i) {
          final video = videos[i];
          final engRate = video.views > 0
              ? (video.likes / video.views * 100).toStringAsFixed(1)
              : '0.0';
          return Padding(
            padding:
                const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            child: Row(
              children: [
                // Rank
                SizedBox(
                  width: 24,
                  child: Text(
                    '${i + 1}',
                    style: const TextStyle(
                      color: _kTextSecondary,
                      fontSize: 13,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                const SizedBox(width: 10),
                // Thumbnail placeholder
                Container(
                  width: 48,
                  height: 48,
                  decoration: BoxDecoration(
                    color: _kSurfaceElevated,
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: const Icon(Icons.play_circle_outline,
                      color: _kTextSecondary, size: 24),
                ),
                const SizedBox(width: 12),
                // Title + stats
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        video.title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: _kTextPrimary,
                          fontSize: 13,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                      const SizedBox(height: 3),
                      Text(
                        '${_fmt(video.views)} views  •  $engRate% eng.',
                        style: const TextStyle(
                          color: _kTextSecondary,
                          fontSize: 11,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Traffic sources pie chart
// ─────────────────────────────────────────────────────────────────────────────

class _TrafficSourcesCard extends StatelessWidget {
  const _TrafficSourcesCard({required this.sources});
  final TrafficSources sources;

  @override
  Widget build(BuildContext context) {
    final slices = [
      ('For You', sources.fyp, _kRed),
      ('Following', sources.following, _kTeal),
      ('Profile', sources.profile, _kOrange),
      ('Search', sources.search, _kBlue),
      ('Other', sources.other, const Color(0xFF9B59B6)),
    ];

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: _kSurface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: _kGrid, width: 1),
      ),
      child: Row(
        children: [
          SizedBox(
            width: 140,
            height: 140,
            child: CustomPaint(
              painter: _PieChartPainter(slices: slices),
            ),
          ),
          const SizedBox(width: 20),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisAlignment: MainAxisAlignment.center,
              children: slices.map((s) {
                final pct = s.$2.toStringAsFixed(1);
                return Padding(
                  padding: const EdgeInsets.symmetric(vertical: 4),
                  child: Row(
                    children: [
                      Container(
                        width: 10,
                        height: 10,
                        decoration: BoxDecoration(
                          color: s.$3,
                          shape: BoxShape.circle,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          s.$1,
                          style: const TextStyle(
                            color: _kTextPrimary,
                            fontSize: 12,
                          ),
                        ),
                      ),
                      Text(
                        '$pct%',
                        style: const TextStyle(
                          color: _kTextSecondary,
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                );
              }).toList(),
            ),
          ),
        ],
      ),
    );
  }
}

class _PieChartPainter extends CustomPainter {
  _PieChartPainter({required this.slices});
  final List<(String, double, Color)> slices;

  @override
  void paint(Canvas canvas, Size size) {
    final total = slices.fold<double>(0, (s, e) => s + e.$2);
    if (total == 0) return;

    final center = Offset(size.width / 2, size.height / 2);
    final radius = size.shortestSide / 2;
    const strokeW = 18.0;

    var startAngle = -math.pi / 2;

    for (final slice in slices) {
      final sweep = 2 * math.pi * (slice.$2 / total);
      canvas.drawArc(
        Rect.fromCircle(center: center, radius: radius - strokeW / 2),
        startAngle,
        sweep - 0.03,
        false,
        Paint()
          ..color = slice.$3
          ..style = PaintingStyle.stroke
          ..strokeWidth = strokeW
          ..strokeCap = StrokeCap.butt,
      );
      startAngle += sweep;
    }
  }

  @override
  bool shouldRepaint(_PieChartPainter old) => old.slices != slices;
}

// ─────────────────────────────────────────────────────────────────────────────
// Revenue section
// ─────────────────────────────────────────────────────────────────────────────

class _RevenueSection extends StatelessWidget {
  const _RevenueSection({required this.points, required this.total});
  final List<DateAmountPoint> points;
  final double total;

  @override
  Widget build(BuildContext context) {
    final viewPoints =
        points.map((p) => DateCountPoint(date: p.date, count: p.amount)).toList();
    return Column(
      children: [
        Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
          decoration: BoxDecoration(
            color: _kSurface,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: _kGrid, width: 1),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Total earned',
                style: TextStyle(color: _kTextSecondary, fontSize: 11),
              ),
              const SizedBox(height: 4),
              Text(
                '\$${total.toStringAsFixed(2)}',
                style: const TextStyle(
                  color: _kOrange,
                  fontSize: 28,
                  fontWeight: FontWeight.w700,
                  letterSpacing: -0.5,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 10),
        _LineChartCard(
          points: viewPoints,
          lineColor: _kOrange,
          label: 'revenue',
        ),
      ],
    );
  }
}
