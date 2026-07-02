// import 'package:cached_network_image/cached_network_image.dart';
// import 'package:flutter/material.dart';
// import 'package:flutter_riverpod/flutter_riverpod.dart';
// import 'package:go_router/go_router.dart';
// import 'package:tiktok_clone/features/messaging/domain/entities/conversation_entity.dart';
// import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';
// import 'package:tiktok_clone/features/messaging/presentation/providers/messaging_provider.dart';

// class InboxScreen extends ConsumerStatefulWidget {
//   const InboxScreen({super.key});

//   @override
//   ConsumerState<InboxScreen> createState() => _InboxScreenState();
// }

// class _InboxScreenState extends ConsumerState<InboxScreen> {
//   bool _searchVisible = false;
//   String _searchQuery = '';
//   final _searchController = TextEditingController();

//   @override
//   void dispose() {
//     _searchController.dispose();
//     super.dispose();
//   }

//   @override
//   Widget build(BuildContext context) {
//     final inboxAsync = ref.watch(inboxProvider);
//     final currentUserId = ref.watch(currentUserIdProvider);

//     return Scaffold(
//       backgroundColor: Colors.black,
//       appBar: AppBar(
//         backgroundColor: Colors.black,
//         elevation: 0,
//         centerTitle: false,
//         title: _searchVisible
//             ? _SearchBar(
//                 controller: _searchController,
//                 onChanged: (v) => setState(() => _searchQuery = v),
//                 onClose: () => setState(() {
//                   _searchVisible = false;
//                   _searchQuery = '';
//                   _searchController.clear();
//                 }),
//               )
//             : const Text(
//                 'Messages',
//                 style: TextStyle(
//                   color: Colors.white,
//                   fontSize: 20,
//                   fontWeight: FontWeight.w700,
//                 ),
//               ),
//         actions: [
//           if (!_searchVisible)
//             IconButton(
//               icon: const Icon(Icons.search, color: Colors.white),
//               onPressed: () => setState(() => _searchVisible = true),
//             ),
//         ],
//       ),
//       body: inboxAsync.when(
//         loading: () => const Center(
//           child: CircularProgressIndicator(
//             valueColor: AlwaysStoppedAnimation(Color(0xFFFE2C55)),
//             strokeWidth: 2,
//           ),
//         ),
//         error: (err, _) => Center(
//           child: Column(
//             mainAxisSize: MainAxisSize.min,
//             children: [
//               const Icon(Icons.error_outline, color: Colors.white38, size: 48),
//               const SizedBox(height: 12),
//               Text(
//                 err.toString(),
//                 style: const TextStyle(color: Colors.white38, fontSize: 14),
//                 textAlign: TextAlign.center,
//               ),
//               const SizedBox(height: 16),
//               TextButton(
//                 onPressed: () => ref.invalidate(inboxProvider),
//                 child: const Text(
//                   'Try again',
//                   style: TextStyle(color: Color(0xFFFE2C55)),
//                 ),
//               ),
//             ],
//           ),
//         ),
//         data: (conversations) {
//           final filtered = _searchQuery.isEmpty
//               ? conversations
//               : conversations.where((c) {
//                   final name = c.displayName(currentUserId).toLowerCase();
//                   return name.contains(_searchQuery.toLowerCase());
//                 }).toList();

//           if (filtered.isEmpty) {
//             return _EmptyState(hasSearch: _searchQuery.isNotEmpty);
//           }

//           return RefreshIndicator(
//             color: const Color(0xFFFE2C55),
//             backgroundColor: const Color(0xFF1A1A1A),
//             onRefresh: () => ref.read(inboxProvider.notifier).refresh(),
//             child: ListView.builder(
//               itemCount: filtered.length,
//               itemBuilder: (context, index) {
//                 final conv = filtered[index];
//                 return _ConversationTile(
//                   conversation: conv,
//                   currentUserId: currentUserId,
//                   onTap: () => context.push(
//                     '/inbox/chat/${conv.id}',
//                     extra: {
//                       'displayName': conv.displayName(currentUserId),
//                       'avatarUrl': conv.avatarUrl(currentUserId),
//                       'isOnline': conv.isOnline(currentUserId),
//                     },
//                   ),
//                 );
//               },
//             ),
//           );
//         },
//       ),
//     );
//   }
// }

// // ---------------------------------------------------------------------------
// // Search bar
// // ---------------------------------------------------------------------------

// class _SearchBar extends StatelessWidget {
//   final TextEditingController controller;
//   final ValueChanged<String> onChanged;
//   final VoidCallback onClose;

//   const _SearchBar({
//     required this.controller,
//     required this.onChanged,
//     required this.onClose,
//   });

//   @override
//   Widget build(BuildContext context) {
//     return Row(
//       children: [
//         Expanded(
//           child: TextField(
//             controller: controller,
//             autofocus: true,
//             onChanged: onChanged,
//             style: const TextStyle(color: Colors.white, fontSize: 16),
//             decoration: const InputDecoration(
//               hintText: 'Search messages...',
//               hintStyle: TextStyle(color: Colors.white38),
//               border: InputBorder.none,
//               prefixIcon: Icon(Icons.search, color: Colors.white38, size: 20),
//             ),
//           ),
//         ),
//         TextButton(
//           onPressed: onClose,
//           child: const Text(
//             'Cancel',
//             style: TextStyle(color: Color(0xFFFE2C55), fontSize: 14),
//           ),
//         ),
//       ],
//     );
//   }
// }

// // ---------------------------------------------------------------------------
// // Conversation tile
// // ---------------------------------------------------------------------------

// class _ConversationTile extends StatelessWidget {
//   final ConversationEntity conversation;
//   final String currentUserId;
//   final VoidCallback onTap;

//   const _ConversationTile({
//     required this.conversation,
//     required this.currentUserId,
//     required this.onTap,
//   });

//   @override
//   Widget build(BuildContext context) {
//     final name = conversation.displayName(currentUserId);
//     final avatarUrl = conversation.avatarUrl(currentUserId);
//     final isOnline = conversation.isOnline(currentUserId);
//     final hasUnread = conversation.unreadCount > 0;

//     return InkWell(
//       onTap: onTap,
//       splashColor: Colors.white10,
//       highlightColor: Colors.white10,
//       child: Padding(
//         padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
//         child: Row(
//           children: [
//             // Avatar with online dot
//             Stack(
//               children: [
//                 CircleAvatar(
//                   radius: 30,
//                   backgroundColor: const Color(0xFF2A2A2A),
//                   backgroundImage: avatarUrl != null
//                       ? CachedNetworkImageProvider(avatarUrl)
//                       : null,
//                   child: avatarUrl == null
//                       ? Text(
//                           name.isNotEmpty ? name[0].toUpperCase() : '?',
//                           style: const TextStyle(
//                             color: Colors.white,
//                             fontSize: 22,
//                             fontWeight: FontWeight.w600,
//                           ),
//                         )
//                       : null,
//                 ),
//                 if (isOnline)
//                   Positioned(
//                     right: 2,
//                     bottom: 2,
//                     child: Container(
//                       width: 12,
//                       height: 12,
//                       decoration: BoxDecoration(
//                         color: const Color(0xFF00D68F),
//                         shape: BoxShape.circle,
//                         border: Border.all(color: Colors.black, width: 2),
//                       ),
//                     ),
//                   ),
//               ],
//             ),
//             const SizedBox(width: 12),
//             // Content
//             Expanded(
//               child: Column(
//                 crossAxisAlignment: CrossAxisAlignment.start,
//                 children: [
//                   Text(
//                     name,
//                     style: TextStyle(
//                       color: Colors.white,
//                       fontSize: 15,
//                       fontWeight: hasUnread ? FontWeight.w700 : FontWeight.w400,
//                     ),
//                   ),
//                   const SizedBox(height: 3),
//                   Text(
//                     _lastMessagePreview(),
//                     maxLines: 1,
//                     overflow: TextOverflow.ellipsis,
//                     style: TextStyle(
//                       color: hasUnread ? Colors.white70 : Colors.white38,
//                       fontSize: 13,
//                       fontWeight: hasUnread ? FontWeight.w500 : FontWeight.w400,
//                     ),
//                   ),
//                 ],
//               ),
//             ),
//             const SizedBox(width: 8),
//             // Trailing: time + badge
//             Column(
//               crossAxisAlignment: CrossAxisAlignment.end,
//               children: [
//                 Text(
//                   _timeAgo(conversation.lastMessageAt),
//                   style: TextStyle(
//                     color: hasUnread ? const Color(0xFFFE2C55) : Colors.white38,
//                     fontSize: 12,
//                   ),
//                 ),
//                 const SizedBox(height: 4),
//                 if (hasUnread)
//                   Container(
//                     constraints: const BoxConstraints(minWidth: 20),
//                     height: 20,
//                     padding: const EdgeInsets.symmetric(horizontal: 6),
//                     decoration: const BoxDecoration(
//                       color: Color(0xFFFE2C55),
//                       borderRadius: BorderRadius.all(Radius.circular(10)),
//                     ),
//                     child: Center(
//                       child: Text(
//                         conversation.unreadCount > 99
//                             ? '99+'
//                             : '${conversation.unreadCount}',
//                         style: const TextStyle(
//                           color: Colors.white,
//                           fontSize: 11,
//                           fontWeight: FontWeight.w700,
//                         ),
//                       ),
//                     ),
//                   )
//                 else
//                   const SizedBox(height: 20),
//               ],
//             ),
//           ],
//         ),
//       ),
//     );
//   }

//   String _lastMessagePreview() {
//     final msg = conversation.lastMessage;
//     if (msg == null) return 'No messages yet';
//     switch (msg.type) {
//       case MessageType.image:
//         return '📷 Photo';
//       case MessageType.video:
//         return '🎥 Video';
//       case MessageType.voice:
//         return '🎤 Voice';
//       case MessageType.gift:
//         return '🎁 Gift';
//       case MessageType.system:
//         return msg.content;
//       case MessageType.text:
//         final c = msg.content;
//         return c.length > 40 ? '${c.substring(0, 40)}…' : c;
//     }
//   }

//   String _timeAgo(DateTime? dt) {
//     if (dt == null) return '';
//     final diff = DateTime.now().difference(dt);
//     if (diff.inMinutes < 1) return 'now';
//     if (diff.inHours < 1) return '${diff.inMinutes}m';
//     if (diff.inDays < 1) return '${diff.inHours}h';
//     if (diff.inDays < 7) return '${diff.inDays}d';
//     return '${dt.day}/${dt.month}';
//   }
// }

// // ---------------------------------------------------------------------------
// // Empty state
// // ---------------------------------------------------------------------------

// class _EmptyState extends StatelessWidget {
//   final bool hasSearch;
//   const _EmptyState({required this.hasSearch});

//   @override
//   Widget build(BuildContext context) {
//     return Center(
//       child: Padding(
//         padding: const EdgeInsets.symmetric(horizontal: 40),
//         child: Column(
//           mainAxisSize: MainAxisSize.min,
//           children: [
//             const Text('💬', style: TextStyle(fontSize: 56)),
//             const SizedBox(height: 16),
//             Text(
//               hasSearch
//                   ? 'No conversations match your search.'
//                   : 'No messages yet.\nFollow creators and start chatting!',
//               style: const TextStyle(
//                 color: Colors.white54,
//                 fontSize: 15,
//                 height: 1.5,
//               ),
//               textAlign: TextAlign.center,
//             ),
//           ],
//         ),
//       ),
//     );
//   }
// }


import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/conversation_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';
import 'package:tiktok_clone/features/messaging/presentation/providers/messaging_provider.dart';

class InboxScreen extends ConsumerStatefulWidget {
  const InboxScreen({super.key});

  @override
  ConsumerState<InboxScreen> createState() => _InboxScreenState();
}

class _InboxScreenState extends ConsumerState<InboxScreen> {
  bool _searchVisible = false;
  String _searchQuery = '';
  final _searchController = TextEditingController();

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _showNewConversationDialog() async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => _NewConversationSheet(
        onConversationCreated: (convId, username) {
          if (mounted) {
            context.push(
              '/inbox/chat/$convId',
              extra: {'displayName': username, 'avatarUrl': null, 'isOnline': false},
            );
          }
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final inboxAsync = ref.watch(inboxProvider);
    final currentUserId = ref.watch(currentUserIdProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      floatingActionButton: FloatingActionButton(
        backgroundColor: const Color(0xFFFE2C55),
        onPressed: _showNewConversationDialog,
        child: const Icon(Icons.edit_outlined, color: Colors.white),
      ),
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        centerTitle: false,
        title: _searchVisible
            ? _SearchBar(
                controller: _searchController,
                onChanged: (v) => setState(() => _searchQuery = v),
                onClose: () => setState(() {
                  _searchVisible = false;
                  _searchQuery = '';
                  _searchController.clear();
                }),
              )
            : const Text(
                'Messages',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 20,
                  fontWeight: FontWeight.w700,
                ),
              ),
        actions: [
          if (!_searchVisible)
            IconButton(
              icon: const Icon(Icons.search, color: Colors.white),
              onPressed: () => setState(() => _searchVisible = true),
            ),
        ],
      ),
      body: inboxAsync.when(
        loading: () => const Center(
          child: CircularProgressIndicator(
            valueColor: AlwaysStoppedAnimation(Color(0xFFFE2C55)),
            strokeWidth: 2,
          ),
        ),
        error: (err, _) => Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.error_outline,
                  color: Colors.white38, size: 48),
              const SizedBox(height: 12),
              Text(
                err.toString(),
                style: const TextStyle(
                    color: Colors.white38, fontSize: 14),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),
              TextButton(
                onPressed: () => ref.invalidate(inboxProvider),
                child: const Text('Try again',
                    style: TextStyle(color: Color(0xFFFE2C55))),
              ),
            ],
          ),
        ),
        data: (conversations) {
          final filtered = _searchQuery.isEmpty
              ? conversations
              : conversations.where((c) {
                  final name =
                      c.displayName(currentUserId).toLowerCase();
                  return name.contains(_searchQuery.toLowerCase());
                }).toList();

          // Online users for the story-style row
          final onlineConvs = conversations
              .where((c) => c.isOnline(currentUserId))
              .toList();

          return RefreshIndicator(
            color: const Color(0xFFFE2C55),
            backgroundColor: const Color(0xFF1A1A1A),
            onRefresh: () => ref.read(inboxProvider.notifier).refresh(),
            child: CustomScrollView(
              slivers: [
                // ── Online users row ──────────────────────────────────
                if (onlineConvs.isNotEmpty && _searchQuery.isEmpty)
                  SliverToBoxAdapter(
                    child: _OnlineUsersRow(
                      conversations: onlineConvs,
                      currentUserId: currentUserId,
                      onTap: (conv) => context.push(
                        '/inbox/chat/${conv.id}',
                        extra: {
                          'displayName':
                              conv.displayName(currentUserId),
                          'avatarUrl': conv.avatarUrl(currentUserId),
                          'isOnline': conv.isOnline(currentUserId),
                        },
                      ),
                    ),
                  ),

                // ── Divider ───────────────────────────────────────────
                if (onlineConvs.isNotEmpty && _searchQuery.isEmpty)
                  const SliverToBoxAdapter(
                    child: Divider(
                        color: Colors.white12, height: 1),
                  ),

                // ── Conversation list ─────────────────────────────────
                filtered.isEmpty
                    ? SliverFillRemaining(
                        child:
                            _EmptyState(hasSearch: _searchQuery.isNotEmpty),
                      )
                    : SliverList(
                        delegate: SliverChildBuilderDelegate(
                          (context, index) {
                            final conv = filtered[index];
                            return _ConversationTile(
                              conversation: conv,
                              currentUserId: currentUserId,
                              onTap: () => context.push(
                                '/inbox/chat/${conv.id}',
                                extra: {
                                  'displayName':
                                      conv.displayName(currentUserId),
                                  'avatarUrl':
                                      conv.avatarUrl(currentUserId),
                                  'isOnline':
                                      conv.isOnline(currentUserId),
                                },
                              ),
                            );
                          },
                          childCount: filtered.length,
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

// ---------------------------------------------------------------------------
// Online users horizontal row (Instagram-style)
// ---------------------------------------------------------------------------

class _OnlineUsersRow extends StatelessWidget {
  final List<ConversationEntity> conversations;
  final String currentUserId;
  final void Function(ConversationEntity) onTap;

  const _OnlineUsersRow({
    required this.conversations,
    required this.currentUserId,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Padding(
          padding: EdgeInsets.fromLTRB(16, 12, 16, 8),
          child: Text(
            'Active Now',
            style: TextStyle(
              color: Colors.white,
              fontSize: 14,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
        SizedBox(
          height: 90,
          child: ListView.builder(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            itemCount: conversations.length,
            itemBuilder: (context, index) {
              final conv = conversations[index];
              final name = conv.displayName(currentUserId);
              final avatarUrl = conv.avatarUrl(currentUserId);

              return GestureDetector(
                onTap: () => onTap(conv),
                child: Container(
                  width: 68,
                  margin: const EdgeInsets.only(right: 8),
                  child: Column(
                    children: [
                      Stack(
                        children: [
                          CircleAvatar(
                            radius: 28,
                            backgroundColor: const Color(0xFF2A2A2A),
                            backgroundImage: avatarUrl != null
                                ? CachedNetworkImageProvider(avatarUrl)
                                : null,
                            child: avatarUrl == null
                                ? Text(
                                    name.isNotEmpty
                                        ? name[0].toUpperCase()
                                        : '?',
                                    style: const TextStyle(
                                      color: Colors.white,
                                      fontSize: 20,
                                      fontWeight: FontWeight.w600,
                                    ),
                                  )
                                : null,
                          ),
                          // Green online dot
                          Positioned(
                            right: 2,
                            bottom: 2,
                            child: Container(
                              width: 14,
                              height: 14,
                              decoration: BoxDecoration(
                                color: const Color(0xFF00D68F),
                                shape: BoxShape.circle,
                                border: Border.all(
                                    color: Colors.black, width: 2.5),
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Text(
                        name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          color: Colors.white70,
                          fontSize: 11,
                        ),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
        const SizedBox(height: 8),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Search bar
// ---------------------------------------------------------------------------

class _SearchBar extends StatelessWidget {
  final TextEditingController controller;
  final ValueChanged<String> onChanged;
  final VoidCallback onClose;

  const _SearchBar({
    required this.controller,
    required this.onChanged,
    required this.onClose,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: TextField(
            controller: controller,
            autofocus: true,
            onChanged: onChanged,
            style: const TextStyle(color: Colors.white, fontSize: 16),
            decoration: const InputDecoration(
              hintText: 'Search messages...',
              hintStyle: TextStyle(color: Colors.white38),
              border: InputBorder.none,
              prefixIcon:
                  Icon(Icons.search, color: Colors.white38, size: 20),
            ),
          ),
        ),
        TextButton(
          onPressed: onClose,
          child: const Text(
            'Cancel',
            style: TextStyle(color: Color(0xFFFE2C55), fontSize: 14),
          ),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Conversation tile
// ---------------------------------------------------------------------------

class _ConversationTile extends StatelessWidget {
  final ConversationEntity conversation;
  final String currentUserId;
  final VoidCallback onTap;

  const _ConversationTile({
    required this.conversation,
    required this.currentUserId,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final name = conversation.displayName(currentUserId);
    final avatarUrl = conversation.avatarUrl(currentUserId);
    final isOnline = conversation.isOnline(currentUserId);
    final hasUnread = conversation.unreadCount > 0;

    return InkWell(
      onTap: onTap,
      splashColor: Colors.white10,
      highlightColor: Colors.white10,
      child: Padding(
        padding:
            const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        child: Row(
          children: [
            // Avatar with online dot
            Stack(
              children: [
                CircleAvatar(
                  radius: 28,
                  backgroundColor: const Color(0xFF2A2A2A),
                  backgroundImage: avatarUrl != null
                      ? CachedNetworkImageProvider(avatarUrl)
                      : null,
                  child: avatarUrl == null
                      ? Text(
                          name.isNotEmpty ? name[0].toUpperCase() : '?',
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 20,
                            fontWeight: FontWeight.w600,
                          ),
                        )
                      : null,
                ),
                if (isOnline)
                  Positioned(
                    right: 2,
                    bottom: 2,
                    child: Container(
                      width: 13,
                      height: 13,
                      decoration: BoxDecoration(
                        color: const Color(0xFF00D68F),
                        shape: BoxShape.circle,
                        border:
                            Border.all(color: Colors.black, width: 2),
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(width: 12),
            // Content
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name,
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 15,
                      fontWeight: hasUnread
                          ? FontWeight.w700
                          : FontWeight.w400,
                    ),
                  ),
                  const SizedBox(height: 3),
                  Text(
                    _lastMessagePreview(),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      color: hasUnread
                          ? Colors.white70
                          : Colors.white38,
                      fontSize: 13,
                      fontWeight: hasUnread
                          ? FontWeight.w500
                          : FontWeight.w400,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            // Trailing: time + badge
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Text(
                  _timeAgo(conversation.lastMessageAt),
                  style: TextStyle(
                    color: hasUnread
                        ? const Color(0xFFFE2C55)
                        : Colors.white38,
                    fontSize: 12,
                  ),
                ),
                const SizedBox(height: 4),
                if (hasUnread)
                  Container(
                    constraints: const BoxConstraints(minWidth: 20),
                    height: 20,
                    padding: const EdgeInsets.symmetric(horizontal: 6),
                    decoration: const BoxDecoration(
                      color: Color(0xFFFE2C55),
                      borderRadius:
                          BorderRadius.all(Radius.circular(10)),
                    ),
                    child: Center(
                      child: Text(
                        conversation.unreadCount > 99
                            ? '99+'
                            : '${conversation.unreadCount}',
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 11,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                  )
                else
                  const SizedBox(height: 20),
              ],
            ),
          ],
        ),
      ),
    );
  }

  String _lastMessagePreview() {
    final msg = conversation.lastMessage;
    if (msg == null) return 'No messages yet';
    switch (msg.type) {
      case MessageType.image:
        return '📷 Photo';
      case MessageType.video:
        return '🎥 Video';
      case MessageType.voice:
        return '🎤 Voice';
      case MessageType.gift:
        return '🎁 Gift';
      case MessageType.system:
        return msg.content;
      case MessageType.text:
        final c = msg.content;
        return c.length > 40 ? '${c.substring(0, 40)}…' : c;
    }
  }

  String _timeAgo(DateTime? dt) {
    if (dt == null) return '';
    final diff = DateTime.now().difference(dt);
    if (diff.inMinutes < 1) return 'now';
    if (diff.inHours < 1) return '${diff.inMinutes}m';
    if (diff.inDays < 1) return '${diff.inHours}h';
    if (diff.inDays < 7) return '${diff.inDays}d';
    return '${dt.day}/${dt.month}';
  }
}

// ---------------------------------------------------------------------------
// Empty state
// ---------------------------------------------------------------------------

class _EmptyState extends StatelessWidget {
  final bool hasSearch;
  const _EmptyState({required this.hasSearch});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Text('💬', style: TextStyle(fontSize: 56)),
            const SizedBox(height: 16),
            Text(
              hasSearch
                  ? 'No conversations match your search.'
                  : 'No messages yet.\nFollow creators and start chatting!',
              style: const TextStyle(
                color: Colors.white54,
                fontSize: 15,
                height: 1.5,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// New conversation bottom sheet
// ---------------------------------------------------------------------------

class _NewConversationSheet extends ConsumerStatefulWidget {
  final void Function(String convId, String username) onConversationCreated;

  const _NewConversationSheet({required this.onConversationCreated});

  @override
  ConsumerState<_NewConversationSheet> createState() =>
      _NewConversationSheetState();
}

class _NewConversationSheetState
    extends ConsumerState<_NewConversationSheet> {
  final _ctrl = TextEditingController();
  List<Map<String, dynamic>> _results = [];
  bool _loading = false;
  bool _creating = false;
  String _error = '';

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  Future<void> _search(String q) async {
    if (q.trim().isEmpty) {
      setState(() { _results = []; _error = ''; });
      return;
    }
    setState(() => _loading = true);
    try {
      final resp = await ApiClient.instance.dio
          .get<Map<String, dynamic>>('/search', queryParameters: {'q': q.trim(), 'type': 'user'});
      final data = resp.data?['data'] as List<dynamic>? ?? [];
      setState(() {
        _results = data.map((e) => e as Map<String, dynamic>).toList();
        _error = '';
      });
    } catch (e) {
      setState(() => _error = 'Search failed');
    } finally {
      setState(() => _loading = false);
    }
  }

  Future<void> _startConversation(Map<String, dynamic> user) async {
    setState(() => _creating = true);
    try {
      final userId = int.tryParse(user['user_id'] as String? ?? user['id'] as String? ?? '0') ?? 0;
      final resp = await ApiClient.instance.dio.post<Map<String, dynamic>>(
        '/conversations',
        data: {'user_id': userId},
      );
      final convId = resp.data?['id'] as String? ?? '';
      final username = user['username'] as String? ?? 'Unknown';
      if (mounted) Navigator.of(context).pop();
      widget.onConversationCreated(convId, username);
    } catch (_) {
      setState(() { _creating = false; _error = 'Could not start conversation'; });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const SizedBox(height: 12),
          Container(
            width: 36,
            height: 4,
            decoration: BoxDecoration(
              color: Colors.white24,
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          const SizedBox(height: 16),
          const Text(
            'New Message',
            style: TextStyle(
              color: Colors.white,
              fontSize: 16,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 12),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: TextField(
              controller: _ctrl,
              autofocus: true,
              style: const TextStyle(color: Colors.white),
              decoration: InputDecoration(
                hintText: 'Search users...',
                hintStyle: const TextStyle(color: Colors.white38),
                filled: true,
                fillColor: const Color(0xFF2A2A2A),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: BorderSide.none,
                ),
                prefixIcon: const Icon(Icons.search,
                    color: Colors.white38),
                suffixIcon: _loading
                    ? const Padding(
                        padding: EdgeInsets.all(12),
                        child: SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Color(0xFFFE2C55),
                          ),
                        ),
                      )
                    : null,
              ),
              onChanged: _search,
            ),
          ),
          if (_error.isNotEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(
                  horizontal: 16, vertical: 8),
              child: Text(_error,
                  style: const TextStyle(
                      color: Colors.redAccent, fontSize: 13)),
            ),
          const SizedBox(height: 8),
          if (_results.isEmpty && !_loading && _ctrl.text.isNotEmpty)
            const Padding(
              padding:
                  EdgeInsets.symmetric(horizontal: 16, vertical: 24),
              child: Text(
                'No users found',
                style: TextStyle(color: Colors.white54, fontSize: 14),
              ),
            )
          else
            ConstrainedBox(
              constraints: BoxConstraints(
                maxHeight:
                    MediaQuery.of(context).size.height * 0.4,
              ),
              child: ListView.builder(
                shrinkWrap: true,
                itemCount: _results.length,
                itemBuilder: (context, index) {
                  final u = _results[index];
                  final username =
                      u['username'] as String? ?? 'Unknown';
                  final avatarUrl = u['avatar_url'] as String?;
                  final displayName =
                      u['display_name'] as String? ?? username;
                  return ListTile(
                    leading: CircleAvatar(
                      backgroundColor: const Color(0xFF2A2A2A),
                      backgroundImage: avatarUrl != null
                          ? NetworkImage(avatarUrl)
                          : null,
                      child: avatarUrl == null
                          ? Text(
                              username.isNotEmpty
                                  ? username[0].toUpperCase()
                                  : '?',
                              style: const TextStyle(
                                  color: Colors.white,
                                  fontWeight: FontWeight.bold),
                            )
                          : null,
                    ),
                    title: Text(
                      displayName,
                      style: const TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.w600),
                    ),
                    subtitle: Text(
                      '@$username',
                      style: const TextStyle(
                          color: Colors.white54, fontSize: 12),
                    ),
                    onTap: _creating ? null : () => _startConversation(u),
                  );
                },
              ),
            ),
          const SizedBox(height: 16),
        ],
      ),
    );
  }
}