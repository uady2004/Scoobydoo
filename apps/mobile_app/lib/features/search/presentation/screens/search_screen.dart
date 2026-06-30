import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:tiktok_clone/features/search/presentation/providers/search_provider.dart';
import 'package:tiktok_clone/features/search/presentation/widgets/trending_card.dart';

// ── Popular topics data ─────────────────────────────────────────────────────

const _kTopics = [
  {'label': 'Dance', 'emoji': '💃', 'color': 0xFFFF0050},
  {'label': 'Comedy', 'emoji': '😂', 'color': 0xFFFF6B35},
  {'label': 'Food', 'emoji': '🍕', 'color': 0xFFFF9500},
  {'label': 'Travel', 'emoji': '✈️', 'color': 0xFF34C759},
  {'label': 'Fitness', 'emoji': '💪', 'color': 0xFF00C9BE},
  {'label': 'Beauty', 'emoji': '💄', 'color': 0xFFFF2D55},
  {'label': 'Tech', 'emoji': '💻', 'color': 0xFF007AFF},
  {'label': 'Music', 'emoji': '🎵', 'color': 0xFF5856D6},
  {'label': 'Fashion', 'emoji': '👗', 'color': 0xFFFF375F},
  {'label': 'Cooking', 'emoji': '🍳', 'color': 0xFFFF8C00},
  {'label': 'Gaming', 'emoji': '🎮', 'color': 0xFF30B0C7},
  {'label': 'Nature', 'emoji': '🌿', 'color': 0xFF4CD964},
];

// ─────────────────────────────────────────────────────────────────────────────
// Screen
// ─────────────────────────────────────────────────────────────────────────────

class SearchScreen extends ConsumerStatefulWidget {
  const SearchScreen({super.key});

  @override
  ConsumerState<SearchScreen> createState() => _SearchScreenState();
}

class _SearchScreenState extends ConsumerState<SearchScreen> {
  final _controller = TextEditingController();
  final _focusNode = FocusNode();

  @override
  void initState() {
    super.initState();
    _focusNode.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  bool get _isSearching => _focusNode.hasFocus;

  void _onSubmit(String q) {
    final query = q.trim();
    if (query.isEmpty) return;
    ref.read(searchProvider.notifier).search(query: query);
    _focusNode.unfocus();
    context.push('/search/results?q=${Uri.encodeComponent(query)}');
  }

  void _onSuggestionTap(String s) {
    _controller.text = s;
    _onSubmit(s);
  }

  void _onHistoryTap(String q) {
    _controller.text = q;
    _onSubmit(q);
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(searchProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: Column(
          children: [
            _SearchBar(
              controller: _controller,
              focusNode: _focusNode,
              query: state.query,
              onChanged: ref.read(searchProvider.notifier).onQueryChanged,
              onSubmitted: _onSubmit,
              onClear: () {
                _controller.clear();
                ref.read(searchProvider.notifier).clearQuery();
              },
              onCancel: () {
                _controller.clear();
                ref.read(searchProvider.notifier).clearQuery();
                _focusNode.unfocus();
              },
            ),

            // Suggestions while typing
            if (_isSearching && state.suggestions.isNotEmpty)
              Expanded(
                child: _SuggestionsPanel(
                  suggestions: state.suggestions,
                  onTap: _onSuggestionTap,
                ),
              )
            // Filter chips + full discover when not typing
            else
              Expanded(
                child: CustomScrollView(
                  slivers: [
                    // ── Filter chips ──────────────────────────────────────
                    SliverToBoxAdapter(
                      child: Padding(
                        padding: const EdgeInsets.only(top: 8, bottom: 4),
                        child: _FilterChips(
                          active: state.activeFilter,
                          onSelected: (f) {
                            ref.read(searchProvider.notifier).setFilter(f);
                            if (state.query.isNotEmpty) _onSubmit(state.query);
                          },
                        ),
                      ),
                    ),

                    // ── Trending Now ──────────────────────────────────────
                    if (state.trending.isNotEmpty) ...[
                      SliverToBoxAdapter(
                        child: _SectionHeader(
                          icon: Icons.local_fire_department_rounded,
                          iconColor: const Color(0xFFFF6B35),
                          title: 'Trending Now',
                          actionLabel: 'See all',
                          onAction: () => context
                              .push('/search/results?q=trending'),
                        ),
                      ),
                      SliverPadding(
                        padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                        sliver: SliverGrid(
                          delegate: SliverChildBuilderDelegate(
                            (_, i) => TrendingCard(item: state.trending[i]),
                            childCount: state.trending.length.clamp(0, 8),
                          ),
                          gridDelegate:
                              const SliverGridDelegateWithFixedCrossAxisCount(
                            crossAxisCount: 2,
                            crossAxisSpacing: 8,
                            mainAxisSpacing: 8,
                            childAspectRatio: 1.65,
                          ),
                        ),
                      ),
                      const SliverToBoxAdapter(child: SizedBox(height: 28)),
                    ],

                    // ── Popular topics ────────────────────────────────────
                    SliverToBoxAdapter(
                      child: _SectionHeader(
                        icon: Icons.tag_rounded,
                        iconColor: const Color(0xFFFF0050),
                        title: 'Popular Topics',
                      ),
                    ),
                    SliverToBoxAdapter(
                      child: Padding(
                        padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                        child: Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          children: _kTopics
                              .map((t) => _TopicChip(
                                    label: t['label'] as String,
                                    emoji: t['emoji'] as String,
                                    color: Color(t['color'] as int),
                                    onTap: () =>
                                        _onSubmit(t['label'] as String),
                                  ))
                              .toList(),
                        ),
                      ),
                    ),
                    const SliverToBoxAdapter(child: SizedBox(height: 28)),

                    // ── Recent searches ───────────────────────────────────
                    if (state.history.isNotEmpty) ...[
                      SliverToBoxAdapter(
                        child: _SectionHeader(
                          icon: Icons.history_rounded,
                          iconColor: Colors.white38,
                          title: 'Recent Searches',
                          actionLabel: 'Clear',
                          onAction: () => ref
                              .read(searchProvider.notifier)
                              .clearHistory(),
                        ),
                      ),
                      SliverList(
                        delegate: SliverChildBuilderDelegate(
                          (_, i) {
                            final h = state.history[i];
                            return ListTile(
                              dense: true,
                              leading: const Icon(Icons.history_rounded,
                                  color: Colors.white38, size: 20),
                              title: Text(h,
                                  style: const TextStyle(
                                      color: Colors.white70, fontSize: 14)),
                              trailing: const Icon(Icons.north_west_rounded,
                                  color: Colors.white24, size: 14),
                              onTap: () => _onHistoryTap(h),
                            );
                          },
                          childCount: state.history.length.clamp(0, 8),
                        ),
                      ),
                    ],

                    const SliverToBoxAdapter(child: SizedBox(height: 32)),
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Search bar
// ─────────────────────────────────────────────────────────────────────────────

class _SearchBar extends StatelessWidget {
  final TextEditingController controller;
  final FocusNode focusNode;
  final String query;
  final ValueChanged<String> onChanged;
  final ValueChanged<String> onSubmitted;
  final VoidCallback onClear;
  final VoidCallback onCancel;

  const _SearchBar({
    required this.controller,
    required this.focusNode,
    required this.query,
    required this.onChanged,
    required this.onSubmitted,
    required this.onClear,
    required this.onCancel,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 12, 12, 4),
      child: Row(
        children: [
          Expanded(
            child: Container(
              height: 44,
              decoration: BoxDecoration(
                color: const Color(0xFF1E1E1E),
                borderRadius: BorderRadius.circular(22),
              ),
              child: TextField(
                controller: controller,
                focusNode: focusNode,
                style: const TextStyle(color: Colors.white, fontSize: 15),
                textInputAction: TextInputAction.search,
                onChanged: onChanged,
                onSubmitted: onSubmitted,
                decoration: InputDecoration(
                  hintText: 'Search videos, users, sounds…',
                  hintStyle:
                      const TextStyle(color: Colors.white38, fontSize: 14),
                  prefixIcon: const Icon(Icons.search_rounded,
                      color: Colors.white38, size: 20),
                  suffixIcon: query.isNotEmpty
                      ? GestureDetector(
                          onTap: onClear,
                          child: const Icon(Icons.cancel_rounded,
                              color: Colors.white38, size: 18),
                        )
                      : null,
                  border: InputBorder.none,
                  contentPadding:
                      const EdgeInsets.symmetric(vertical: 12),
                ),
              ),
            ),
          ),
          if (focusNode.hasFocus) ...[
            const SizedBox(width: 10),
            GestureDetector(
              onTap: onCancel,
              child: const Text('Cancel',
                  style: TextStyle(color: Colors.white70, fontSize: 15)),
            ),
          ],
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Suggestions panel
// ─────────────────────────────────────────────────────────────────────────────

class _SuggestionsPanel extends StatelessWidget {
  final List<String> suggestions;
  final ValueChanged<String> onTap;

  const _SuggestionsPanel(
      {required this.suggestions, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: const Color(0xFF111111),
      child: ListView.separated(
        padding: EdgeInsets.zero,
        itemCount: suggestions.length,
        separatorBuilder: (_, __) =>
            const Divider(height: 1, color: Colors.white10),
        itemBuilder: (_, i) {
          final s = suggestions[i];
          return ListTile(
            dense: true,
            leading: const Icon(Icons.search_rounded,
                color: Colors.white38, size: 18),
            title: Text(s,
                style: const TextStyle(
                    color: Colors.white70, fontSize: 14)),
            trailing: const Icon(Icons.north_west_rounded,
                color: Colors.white24, size: 14),
            onTap: () => onTap(s),
          );
        },
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Filter chips
// ─────────────────────────────────────────────────────────────────────────────

class _FilterChips extends StatelessWidget {
  final SearchFilterType active;
  final ValueChanged<SearchFilterType> onSelected;

  const _FilterChips({required this.active, required this.onSelected});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 36,
      child: ListView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 12),
        children: SearchFilterType.values.map((f) {
          final isActive = f == active;
          return GestureDetector(
            onTap: () => onSelected(f),
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 180),
              margin: const EdgeInsets.only(right: 8),
              padding: const EdgeInsets.symmetric(
                  horizontal: 18, vertical: 6),
              decoration: BoxDecoration(
                color: isActive
                    ? const Color(0xFFFF0050)
                    : const Color(0xFF1E1E1E),
                borderRadius: BorderRadius.circular(20),
                border: isActive
                    ? null
                    : Border.all(color: Colors.white12),
              ),
              child: Text(
                f.label,
                style: TextStyle(
                  color: isActive ? Colors.white : Colors.white60,
                  fontWeight: isActive
                      ? FontWeight.w700
                      : FontWeight.w500,
                  fontSize: 13,
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
// Section header
// ─────────────────────────────────────────────────────────────────────────────

class _SectionHeader extends StatelessWidget {
  final IconData icon;
  final Color iconColor;
  final String title;
  final String? actionLabel;
  final VoidCallback? onAction;

  const _SectionHeader({
    required this.icon,
    required this.iconColor,
    required this.title,
    this.actionLabel,
    this.onAction,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 0, 8, 0),
      child: Row(
        children: [
          Container(
            width: 28,
            height: 28,
            decoration: BoxDecoration(
              color: iconColor.withValues(alpha: 0.15),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(icon, color: iconColor, size: 16),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              title,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 16,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
          if (actionLabel != null && onAction != null)
            TextButton(
              onPressed: onAction,
              style: TextButton.styleFrom(
                padding: const EdgeInsets.symmetric(horizontal: 8),
                minimumSize: Size.zero,
                tapTargetSize: MaterialTapTargetSize.shrinkWrap,
              ),
              child: Text(
                actionLabel!,
                style: const TextStyle(
                    color: Color(0xFFFF0050), fontSize: 13),
              ),
            ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Topic chip
// ─────────────────────────────────────────────────────────────────────────────

class _TopicChip extends StatelessWidget {
  final String label;
  final String emoji;
  final Color color;
  final VoidCallback onTap;

  const _TopicChip({
    required this.label,
    required this.emoji,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: color.withValues(alpha: 0.35)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(emoji, style: const TextStyle(fontSize: 14)),
            const SizedBox(width: 5),
            Text(
              label,
              style: TextStyle(
                  color: color,
                  fontSize: 13,
                  fontWeight: FontWeight.w600),
            ),
          ],
        ),
      ),
    );
  }
}
