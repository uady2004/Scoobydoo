import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:tiktok_clone/features/search/presentation/providers/search_provider.dart';

class SearchResultsScreen extends ConsumerStatefulWidget {
  const SearchResultsScreen({
    super.key,
    required this.query,
    this.type,
  });

  final String query;
  final String? type;

  @override
  ConsumerState<SearchResultsScreen> createState() =>
      _SearchResultsScreenState();
}

class _SearchResultsScreenState extends ConsumerState<SearchResultsScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;
  late final TextEditingController _searchCtrl;
  late String _query;

  static const _kRed = Color(0xFFFF0050);
  static const _tabs = ['Top', 'Videos', 'Users', 'Hashtags', 'Sounds'];

  static const _typeToTab = {
    'videos': 1,
    'users': 2,
    'hashtags': 3,
    'sounds': 4,
  };

  @override
  void initState() {
    super.initState();
    _query = widget.query;
    _searchCtrl = TextEditingController(text: widget.query);
    _tabController = TabController(
      length: _tabs.length,
      vsync: this,
      initialIndex: _typeToTab[widget.type] ?? 0,
    );
    if (_query.isNotEmpty) {
      ref.read(searchProvider.notifier).search(query: _query);
    }
  }

  @override
  void dispose() {
    _tabController.dispose();
    _searchCtrl.dispose();
    super.dispose();
  }

  void _onSubmitted(String value) {
    final trimmed = value.trim();
    if (trimmed.isEmpty) return;
    setState(() => _query = trimmed);
    ref.read(searchProvider.notifier).search(query: trimmed);
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(searchProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: const Color(0xFF0A0A0A),
        elevation: 0.5,
        iconTheme: const IconThemeData(color: Colors.white),
        titleSpacing: 0,
        title: Container(
          height: 40,
          decoration: BoxDecoration(
            color: const Color(0xFF1E1E1E),
            borderRadius: BorderRadius.circular(20),
          ),
          child: TextField(
            controller: _searchCtrl,
            style: const TextStyle(color: Colors.white, fontSize: 15),
            textInputAction: TextInputAction.search,
            onSubmitted: _onSubmitted,
            decoration: InputDecoration(
              hintText: 'Search…',
              hintStyle:
                  const TextStyle(color: Colors.white38, fontSize: 14),
              prefixIcon: const Icon(Icons.search_rounded,
                  color: Colors.white38, size: 19),
              suffixIcon: IconButton(
                icon: const Icon(Icons.close_rounded,
                    color: Colors.white38, size: 18),
                onPressed: () {
                  _searchCtrl.clear();
                  context.pop();
                },
              ),
              border: InputBorder.none,
              contentPadding:
                  const EdgeInsets.symmetric(vertical: 10),
            ),
          ),
        ),
        actions: const [SizedBox(width: 12)],
        bottom: TabBar(
          controller: _tabController,
          isScrollable: true,
          tabAlignment: TabAlignment.start,
          indicatorColor: _kRed,
          indicatorWeight: 2.5,
          labelColor: Colors.white,
          unselectedLabelColor: Colors.white38,
          labelStyle: const TextStyle(
              fontSize: 14, fontWeight: FontWeight.w600),
          unselectedLabelStyle: const TextStyle(fontSize: 14),
          tabs: _tabs.map((t) => Tab(text: t)).toList(),
        ),
      ),
      body: state.isSearching
          ? const Center(
              child: CircularProgressIndicator(
                  color: _kRed, strokeWidth: 2))
          : state.error != null && state.results.isEmpty
              ? _ErrorState(
                  message: state.error!,
                  onRetry: () => ref
                      .read(searchProvider.notifier)
                      .search(query: _query),
                )
              : TabBarView(
                  controller: _tabController,
                  children: [
                    _TopTab(query: _query, results: state.results),
                    _VideosTab(results: state.results),
                    _UsersTab(results: state.results),
                    _HashtagsTab(results: state.results, query: _query),
                    _SoundsTab(results: state.results, query: _query),
                  ],
                ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

String _fmtCount(int n) {
  if (n >= 1000000000) return '${(n / 1000000000).toStringAsFixed(1)}B';
  if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
  if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
  return '$n';
}

List<Map<String, dynamic>> _videos(List<Map<String, dynamic>> results) =>
    results
        .where((r) => r['result_type'] == 'video')
        .toList();

List<Map<String, dynamic>> _users(List<Map<String, dynamic>> results) =>
    results
        .where((r) => r['result_type'] == 'user')
        .toList();

// ─────────────────────────────────────────────────────────────────────────────
// Empty / error states
// ─────────────────────────────────────────────────────────────────────────────

class _EmptyState extends StatelessWidget {
  final String label;
  const _EmptyState({required this.label});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.search_off_rounded, color: Colors.white24, size: 56),
          const SizedBox(height: 12),
          Text(label,
              style: const TextStyle(color: Colors.white38, fontSize: 14)),
        ],
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;
  const _ErrorState({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.error_outline_rounded,
              color: Colors.white38, size: 48),
          const SizedBox(height: 12),
          Text(message,
              style: const TextStyle(color: Colors.white38, fontSize: 13)),
          const SizedBox(height: 16),
          TextButton(
            onPressed: onRetry,
            child: const Text('Retry',
                style: TextStyle(color: Color(0xFFFF0050))),
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Video card (grid)
// ─────────────────────────────────────────────────────────────────────────────

class _VideoCard extends StatelessWidget {
  final Map<String, dynamic> video;

  const _VideoCard({required this.video});

  static const _gradients = [
    [Color(0xFFFF0050), Color(0xFFFF6B35)],
    [Color(0xFF007AFF), Color(0xFF00C9BE)],
    [Color(0xFF5856D6), Color(0xFFFF2D55)],
    [Color(0xFF34C759), Color(0xFF00F2EA)],
    [Color(0xFFFF9500), Color(0xFFFF375F)],
    [Color(0xFF30B0C7), Color(0xFF5856D6)],
  ];

  @override
  Widget build(BuildContext context) {
    final thumbUrl = video['thumbnail_url'] as String? ?? '';
    final views = (video['view_count'] as num?)?.toInt() ?? 0;
    final creator = '@${video['creator_username'] ?? ''}';
    final idx = (video['video_id'] as String? ?? '').hashCode.abs() % 6;

    return GestureDetector(
      onTap: () {
        final id = video['video_id'] as String? ?? '';
        if (id.isNotEmpty) context.push('/video/$id');
      },
      child: ClipRRect(
        borderRadius: BorderRadius.circular(6),
        child: Stack(
          fit: StackFit.expand,
          children: [
            // Thumbnail or gradient placeholder
            thumbUrl.isNotEmpty
                ? Image.network(
                    thumbUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (_, __, ___) => _GradientBox(
                        colors: _gradients[idx]),
                  )
                : _GradientBox(colors: _gradients[idx]),

            // Bottom gradient
            Positioned.fill(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    stops: const [0.5, 1.0],
                    colors: [
                      Colors.transparent,
                      Colors.black.withValues(alpha: 0.75),
                    ],
                  ),
                ),
              ),
            ),

            // View count — top left
            Positioned(
              top: 6,
              left: 6,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.play_arrow_rounded,
                      color: Colors.white, size: 13),
                  const SizedBox(width: 2),
                  Text(
                    _fmtCount(views),
                    style: const TextStyle(
                        color: Colors.white,
                        fontSize: 10,
                        fontWeight: FontWeight.w700,
                        shadows: [
                          Shadow(blurRadius: 4, color: Colors.black87)
                        ]),
                  ),
                ],
              ),
            ),

            // Creator — bottom left
            Positioned(
              bottom: 6,
              left: 6,
              right: 6,
              child: Text(
                creator,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 10,
                  fontWeight: FontWeight.w600,
                  shadows: [Shadow(blurRadius: 4, color: Colors.black87)],
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _GradientBox extends StatelessWidget {
  final List<Color> colors;
  const _GradientBox({required this.colors});

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: colors,
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// User row (list)
// ─────────────────────────────────────────────────────────────────────────────

class _UserRow extends StatefulWidget {
  final Map<String, dynamic> user;
  const _UserRow({required this.user});

  @override
  State<_UserRow> createState() => _UserRowState();
}

class _UserRowState extends State<_UserRow> {
  late bool _following;

  @override
  void initState() {
    super.initState();
    _following = widget.user['is_following'] as bool? ?? false;
  }

  @override
  Widget build(BuildContext context) {
    final u = widget.user;
    final username = u['username'] as String? ?? '';
    final displayName = u['display_name'] as String? ?? username;
    final avatarUrl = u['avatar_url'] as String? ?? '';
    final followers = (u['follower_count'] as num?)?.toInt() ?? 0;
    final isVerified = u['is_verified'] as bool? ?? false;
    final bio = u['bio'] as String? ?? '';

    return ListTile(
      contentPadding:
          const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
      onTap: () => context.push('/profile/$username'),
      leading: Stack(
        clipBehavior: Clip.none,
        children: [
          CircleAvatar(
            radius: 26,
            backgroundColor: const Color(0xFF2A2A2A),
            backgroundImage:
                avatarUrl.isNotEmpty ? NetworkImage(avatarUrl) : null,
            child: avatarUrl.isEmpty
                ? Text(
                    displayName.isNotEmpty
                        ? displayName[0].toUpperCase()
                        : '?',
                    style: const TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.bold,
                        fontSize: 18))
                : null,
          ),
          if (isVerified)
            Positioned(
              bottom: -2,
              right: -2,
              child: Container(
                width: 16,
                height: 16,
                decoration: const BoxDecoration(
                  color: Color(0xFF20D5EC),
                  shape: BoxShape.circle,
                ),
                child: const Icon(Icons.check,
                    color: Colors.white, size: 10),
              ),
            ),
        ],
      ),
      title: Row(
        children: [
          Text(
            displayName,
            style: const TextStyle(
                color: Colors.white,
                fontSize: 14,
                fontWeight: FontWeight.w600),
          ),
        ],
      ),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('@$username',
              style:
                  const TextStyle(color: Colors.white38, fontSize: 12)),
          if (bio.isNotEmpty)
            Text(bio,
                style:
                    const TextStyle(color: Colors.white54, fontSize: 12),
                maxLines: 1,
                overflow: TextOverflow.ellipsis),
          Text('${_fmtCount(followers)} followers',
              style: const TextStyle(
                  color: Colors.white38, fontSize: 11)),
        ],
      ),
      isThreeLine: bio.isNotEmpty,
      trailing: GestureDetector(
        onTap: () => setState(() => _following = !_following),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          height: 32,
          width: 80,
          decoration: BoxDecoration(
            color: _following
                ? Colors.transparent
                : const Color(0xFFFF0050),
            borderRadius: BorderRadius.circular(6),
            border: _following
                ? Border.all(color: Colors.white30)
                : null,
          ),
          alignment: Alignment.center,
          child: Text(
            _following ? 'Following' : 'Follow',
            style: TextStyle(
              color: _following ? Colors.white60 : Colors.white,
              fontSize: 13,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Top tab
// ─────────────────────────────────────────────────────────────────────────────

class _TopTab extends StatelessWidget {
  final String query;
  final List<Map<String, dynamic>> results;

  const _TopTab({required this.query, required this.results});

  @override
  Widget build(BuildContext context) {
    final users = _users(results);
    final videos = _videos(results);

    if (results.isEmpty) {
      return _EmptyState(label: 'No results for "$query"');
    }

    return CustomScrollView(
      slivers: [
        // ── Users horizontal row ──────────────────────────────────────
        if (users.isNotEmpty) ...[
          const SliverToBoxAdapter(
            child: Padding(
              padding: EdgeInsets.fromLTRB(16, 16, 16, 8),
              child: Text('Users',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 14,
                      fontWeight: FontWeight.w700)),
            ),
          ),
          SliverToBoxAdapter(
            child: SizedBox(
              height: 108,
              child: ListView.builder(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 12),
                itemCount: users.length,
                itemBuilder: (_, i) => _TopUserCard(user: users[i]),
              ),
            ),
          ),
        ],

        // ── Videos section header ────────────────────────────────────
        if (videos.isNotEmpty)
          const SliverToBoxAdapter(
            child: Padding(
              padding: EdgeInsets.fromLTRB(16, 16, 16, 8),
              child: Text('Videos',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 14,
                      fontWeight: FontWeight.w700)),
            ),
          ),

        // ── Video grid ────────────────────────────────────────────────
        SliverPadding(
          padding: const EdgeInsets.fromLTRB(2, 0, 2, 2),
          sliver: SliverGrid(
            delegate: SliverChildBuilderDelegate(
              (_, i) => _VideoCard(video: videos[i]),
              childCount: videos.length,
            ),
            gridDelegate:
                const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 3,
              crossAxisSpacing: 2,
              mainAxisSpacing: 2,
              childAspectRatio: 0.6,
            ),
          ),
        ),
      ],
    );
  }
}

class _TopUserCard extends StatefulWidget {
  final Map<String, dynamic> user;
  const _TopUserCard({required this.user});

  @override
  State<_TopUserCard> createState() => _TopUserCardState();
}

class _TopUserCardState extends State<_TopUserCard> {
  late bool _following;

  @override
  void initState() {
    super.initState();
    _following = widget.user['is_following'] as bool? ?? false;
  }

  @override
  Widget build(BuildContext context) {
    final u = widget.user;
    final username = u['username'] as String? ?? '';
    final displayName = u['display_name'] as String? ?? username;
    final avatarUrl = u['avatar_url'] as String? ?? '';

    return GestureDetector(
      onTap: () => context.push('/profile/$username'),
      child: Container(
        width: 80,
        margin: const EdgeInsets.only(right: 12),
        child: Column(
          children: [
            CircleAvatar(
              radius: 28,
              backgroundColor: const Color(0xFF2A2A2A),
              backgroundImage: avatarUrl.isNotEmpty
                  ? NetworkImage(avatarUrl)
                  : null,
              child: avatarUrl.isEmpty
                  ? Text(
                      displayName.isNotEmpty
                          ? displayName[0].toUpperCase()
                          : '?',
                      style: const TextStyle(
                          color: Colors.white,
                          fontSize: 18,
                          fontWeight: FontWeight.bold))
                  : null,
            ),
            const SizedBox(height: 4),
            Text(
              '@$username',
              style:
                  const TextStyle(color: Colors.white, fontSize: 10),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 4),
            GestureDetector(
              onTap: () => setState(() => _following = !_following),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 180),
                height: 22,
                width: 64,
                decoration: BoxDecoration(
                  color: _following
                      ? Colors.transparent
                      : const Color(0xFFFF0050),
                  borderRadius: BorderRadius.circular(4),
                  border: _following
                      ? Border.all(color: Colors.white30)
                      : null,
                ),
                alignment: Alignment.center,
                child: Text(
                  _following ? 'Following' : 'Follow',
                  style: TextStyle(
                    color:
                        _following ? Colors.white60 : Colors.white,
                    fontSize: 9,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Videos tab
// ─────────────────────────────────────────────────────────────────────────────

class _VideosTab extends StatelessWidget {
  final List<Map<String, dynamic>> results;
  const _VideosTab({required this.results});

  @override
  Widget build(BuildContext context) {
    final videos = _videos(results);
    if (videos.isEmpty) {
      return const _EmptyState(label: 'No videos found');
    }
    return GridView.builder(
      padding: const EdgeInsets.all(2),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        crossAxisSpacing: 2,
        mainAxisSpacing: 2,
        childAspectRatio: 0.6,
      ),
      itemCount: videos.length,
      itemBuilder: (_, i) => _VideoCard(video: videos[i]),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Users tab
// ─────────────────────────────────────────────────────────────────────────────

class _UsersTab extends StatelessWidget {
  final List<Map<String, dynamic>> results;
  const _UsersTab({required this.results});

  @override
  Widget build(BuildContext context) {
    final users = _users(results);
    if (users.isEmpty) {
      return const _EmptyState(label: 'No users found');
    }
    return ListView.separated(
      itemCount: users.length,
      separatorBuilder: (_, __) =>
          const Divider(height: 1, color: Colors.white10, indent: 72),
      itemBuilder: (_, i) => _UserRow(user: users[i]),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Hashtags tab
// ─────────────────────────────────────────────────────────────────────────────

class _HashtagsTab extends StatelessWidget {
  final List<Map<String, dynamic>> results;
  final String query;
  const _HashtagsTab({required this.results, required this.query});

  List<Map<String, dynamic>> _buildHashtags() {
    final seen = <String>{};
    final out = <Map<String, dynamic>>[];

    for (final v in _videos(results)) {
      final raw = v['hashtags'];
      if (raw is List) {
        for (final tag in raw) {
          final t = tag.toString();
          if (seen.add(t)) {
            final count = (v['view_count'] as num?)?.toInt() ?? 1000;
            out.add({'tag': t, 'video_count': count ~/ 10});
          }
        }
      }
    }

    // Fallback generated hashtags when no video results
    if (out.isEmpty && query.isNotEmpty) {
      for (final suffix in ['', 'challenge', 'trend', 'viral', 'fyp']) {
        final tag = suffix.isEmpty ? query : '$query$suffix';
        out.add({'tag': tag, 'video_count': 10000 + tag.length * 2345});
      }
    }

    return out;
  }

  @override
  Widget build(BuildContext context) {
    final hashtags = _buildHashtags();

    if (hashtags.isEmpty) {
      return const _EmptyState(label: 'No hashtags found');
    }

    return ListView.separated(
      itemCount: hashtags.length,
      separatorBuilder: (_, __) =>
          const Divider(height: 1, color: Colors.white10, indent: 72),
      itemBuilder: (_, i) {
        final tag = hashtags[i]['tag'] as String;
        final count = hashtags[i]['video_count'] as int;
        return ListTile(
          contentPadding:
              const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
          onTap: () => context.push('/hashtag/$tag'),
          leading: Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              gradient: LinearGradient(
                colors: [
                  const Color(0xFFFF0050).withValues(alpha: 0.8),
                  const Color(0xFF5856D6).withValues(alpha: 0.8),
                ],
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
              ),
              borderRadius: BorderRadius.circular(10),
            ),
            alignment: Alignment.center,
            child: const Text('#',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 22,
                    fontWeight: FontWeight.w800)),
          ),
          title: Text('#$tag',
              style: const TextStyle(
                  color: Colors.white,
                  fontSize: 14,
                  fontWeight: FontWeight.w600)),
          subtitle: Text('${_fmtCount(count)} videos',
              style:
                  const TextStyle(color: Colors.white38, fontSize: 12)),
          trailing: const Icon(Icons.arrow_forward_ios_rounded,
              color: Colors.white24, size: 14),
        );
      },
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Sounds tab
// ─────────────────────────────────────────────────────────────────────────────

class _SoundsTab extends StatelessWidget {
  final List<Map<String, dynamic>> results;
  final String query;
  const _SoundsTab({required this.results, required this.query});

  List<Map<String, dynamic>> _buildSounds() {
    final seen = <String>{};
    final out = <Map<String, dynamic>>[];

    for (final v in _videos(results)) {
      final title = v['sound_title'] as String? ?? '';
      final artist = v['sound_artist'] as String? ?? '';
      final key = '$title|$artist';
      if (title.isNotEmpty && seen.add(key)) {
        out.add({
          'title': title,
          'artist': artist,
          'video_count': (v['view_count'] as num?)?.toInt() ?? 0,
          'sound_id': v['video_id'] as String? ?? '',
        });
      }
    }

    // Fallback when no video results
    if (out.isEmpty && query.isNotEmpty) {
      out.addAll([
        {'title': 'Original Sound - $query', 'artist': 'Various Artists', 'video_count': 12300, 'sound_id': 's1'},
        {'title': '$query Remix', 'artist': 'DJ Mix', 'video_count': 5600, 'sound_id': 's2'},
        {'title': '${query[0].toUpperCase()}${query.substring(1)} Beats', 'artist': 'Beat Producer', 'video_count': 3200, 'sound_id': 's3'},
      ]);
    }

    return out;
  }

  @override
  Widget build(BuildContext context) {
    final sounds = _buildSounds();

    if (sounds.isEmpty) {
      return const _EmptyState(label: 'No sounds found');
    }

    return ListView.separated(
      itemCount: sounds.length,
      separatorBuilder: (_, __) =>
          const Divider(height: 1, color: Colors.white10, indent: 80),
      itemBuilder: (_, i) {
        final s = sounds[i];
        final title = s['title'] as String;
        final artist = s['artist'] as String;
        final count = (s['video_count'] as num?)?.toInt() ?? 0;
        final soundId = s['sound_id'] as String;

        return ListTile(
          contentPadding:
              const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
          onTap: () {
            if (soundId.isNotEmpty) context.push('/sound/$soundId');
          },
          leading: Container(
            width: 52,
            height: 52,
            decoration: BoxDecoration(
              gradient: LinearGradient(
                colors: [
                  const Color(0xFF5856D6).withValues(alpha: 0.9),
                  const Color(0xFFFF2D55).withValues(alpha: 0.9),
                ],
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
              ),
              borderRadius: BorderRadius.circular(10),
            ),
            child: const Icon(Icons.music_note_rounded,
                color: Colors.white, size: 26),
          ),
          title: Text(title,
              style: const TextStyle(
                  color: Colors.white,
                  fontSize: 14,
                  fontWeight: FontWeight.w600),
              maxLines: 1,
              overflow: TextOverflow.ellipsis),
          subtitle: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(artist,
                  style: const TextStyle(
                      color: Colors.white54, fontSize: 12)),
              Text('${_fmtCount(count)} videos',
                  style: const TextStyle(
                      color: Colors.white38, fontSize: 11)),
            ],
          ),
          isThreeLine: true,
          trailing: _UseButton(soundId: soundId),
        );
      },
    );
  }
}

class _UseButton extends StatefulWidget {
  final String soundId;
  const _UseButton({required this.soundId});

  @override
  State<_UseButton> createState() => _UseButtonState();
}

class _UseButtonState extends State<_UseButton> {
  bool _added = false;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () {
        setState(() => _added = !_added);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(_added ? 'Sound added to favourites' : 'Sound removed'),
            backgroundColor: const Color(0xFF1A1A1A),
            behavior: SnackBarBehavior.floating,
            duration: const Duration(seconds: 1),
          ),
        );
      },
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        height: 32,
        width: 64,
        decoration: BoxDecoration(
          color: _added
              ? const Color(0xFF1E1E1E)
              : const Color(0xFFFF0050),
          borderRadius: BorderRadius.circular(6),
          border: _added ? Border.all(color: Colors.white24) : null,
        ),
        alignment: Alignment.center,
        child: Text(
          _added ? 'Saved' : 'Use',
          style: TextStyle(
            color: _added ? Colors.white54 : Colors.white,
            fontSize: 13,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }
}
