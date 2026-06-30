import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:shimmer/shimmer.dart';

import '../../../home_feed/domain/usecases/report_view_usecase.dart';
import '../../../home_feed/presentation/providers/feed_provider.dart';
import '../../../video_player/presentation/widgets/tiktok_video_player.dart';
import '../../../shares/presentation/screens/share_sheet.dart';

class ForYouScreen extends ConsumerStatefulWidget {
  const ForYouScreen({super.key});

  @override
  ConsumerState<ForYouScreen> createState() => _ForYouScreenState();
}

class _ForYouScreenState extends ConsumerState<ForYouScreen>
    with AutomaticKeepAliveClientMixin {
  late final PageController _pageController;
  int _currentPage = 0;
  DateTime? _pageStartTime;

  @override
  bool get wantKeepAlive => true;

  @override
  void initState() {
    super.initState();
    _pageController = PageController(viewportFraction: 1.0);
    _pageStartTime = DateTime.now();
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  void _onPageChanged(int index) {
    final feedState = ref.read(forYouFeedProvider).valueOrNull;
    if (feedState == null) return;

    // Report the view for the page we are leaving.
    final previous = feedState.items.elementAtOrNull(_currentPage);
    if (previous != null && _pageStartTime != null) {
      final watchSecs =
          DateTime.now().difference(_pageStartTime!).inSeconds;
      final completion =
          previous.duration > 0 ? watchSecs / previous.duration : 0.0;

      ref.read(reportViewUseCaseProvider).call(
            ReportViewParams(
              videoId: previous.videoId,
              watchDuration: watchSecs,
              completionPct: completion.clamp(0.0, 1.0),
            ),
          );
    }

    _currentPage = index;
    _pageStartTime = DateTime.now();

    // Trigger pagination pre-fetch via notifier threshold logic.
    ref.read(forYouFeedProvider.notifier).onPageChanged(index);
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    final feedAsync = ref.watch(forYouFeedProvider);

    return feedAsync.when(
      loading: () => const _ShimmerFeed(),
      error: (err, __) => _ErrorView(
        message: err.toString(),
        onRetry: () => ref.read(forYouFeedProvider.notifier).loadInitial(),
      ),
      data: (feedState) {
        if (feedState.items.isEmpty) {
          return const _EmptyForYou();
        }

        return PageView.builder(
          controller: _pageController,
          scrollDirection: Axis.vertical,
          onPageChanged: _onPageChanged,
          itemCount: feedState.hasReachedEnd
              ? feedState.items.length
              : feedState.items.length + 1,
          itemBuilder: (context, index) {
            // Loading shimmer page while fetching more.
            if (index >= feedState.items.length) {
              return const _ShimmerPage();
            }

            final item = feedState.items[index];
            return TikTokVideoPlayer(
              key: ValueKey(item.videoId),
              item: item,
              isActive: index == _currentPage,
              feedType: 'forYou',
              onCommentTap: () => context.push('/comments/${item.videoId}'),
              onShareTap: () => ShareSheet.show(
                context,
                videoId: item.videoId,
                videoUrl: item.videoUrl,
                videoTitle: item.title,
              ),
              onSoundTap: () {
                final soundParam = item.soundTitle.isNotEmpty
                    ? Uri.encodeComponent(item.soundTitle)
                    : item.videoId;
                context.push('/sound/$soundParam');
              },
              onUsernameTap: () =>
                  context.push('/profile/${item.creatorUsername}'),
              onHashtagTap: (tag) =>
                  context.push('/hashtag/${Uri.encodeComponent(tag)}'),
            );
          },
        );
      },
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Loading / error / empty states
// ─────────────────────────────────────────────────────────────────────────────

class _ShimmerFeed extends StatelessWidget {
  const _ShimmerFeed();

  @override
  Widget build(BuildContext context) => const _ShimmerPage();
}

class _ShimmerPage extends StatelessWidget {
  const _ShimmerPage();

  @override
  Widget build(BuildContext context) {
    return Shimmer.fromColors(
      baseColor: Colors.grey.shade900,
      highlightColor: Colors.grey.shade800,
      child: Container(color: Colors.black),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: Colors.black,
      child: Center(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.error_outline, color: Colors.white54, size: 48),
              const SizedBox(height: 16),
              Text(
                message,
                style: const TextStyle(color: Colors.white70, fontSize: 14),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: onRetry,
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFFF0050),
                  foregroundColor: Colors.white,
                ),
                child: const Text('Try again'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _EmptyForYou extends StatelessWidget {
  const _EmptyForYou();

  @override
  Widget build(BuildContext context) {
    return const ColoredBox(
      color: Colors.black,
      child: Center(
        child: Padding(
          padding: EdgeInsets.symmetric(horizontal: 32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.play_circle_outline, color: Colors.white54, size: 64),
              SizedBox(height: 16),
              Text(
                'Nothing to watch right now',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 18,
                  fontWeight: FontWeight.bold,
                ),
                textAlign: TextAlign.center,
              ),
              SizedBox(height: 8),
              Text(
                'Check back soon — new videos are on their way.',
                style: TextStyle(color: Colors.white60, fontSize: 14),
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
