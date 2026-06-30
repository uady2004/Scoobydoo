import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../ecommerce/domain/entities/order_entity.dart';
import '../../../ecommerce/presentation/providers/ecommerce_provider.dart';
import 'order_detail_screen.dart';

// ─────────────────────────────────────────────────────────────────────────────
// OrdersScreen
// ─────────────────────────────────────────────────────────────────────────────

class OrdersScreen extends ConsumerStatefulWidget {
  const OrdersScreen({super.key});

  @override
  ConsumerState<OrdersScreen> createState() => _OrdersScreenState();
}

class _OrdersScreenState extends ConsumerState<OrdersScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back, color: Colors.white),
          onPressed: () => Navigator.of(context).pop(),
        ),
        title: const Text(
          'My Orders',
          style: TextStyle(
              color: Colors.white,
              fontSize: 18,
              fontWeight: FontWeight.w700),
        ),
        bottom: TabBar(
          controller: _tabController,
          indicatorColor: const Color(0xFFFF2D55),
          indicatorWeight: 2,
          labelColor: const Color(0xFFFF2D55),
          unselectedLabelColor: const Color(0xFF666666),
          labelStyle: const TextStyle(
              fontSize: 13, fontWeight: FontWeight.w600),
          unselectedLabelStyle: const TextStyle(
              fontSize: 13, fontWeight: FontWeight.w400),
          tabs: const [
            Tab(text: 'Active'),
            Tab(text: 'Completed'),
            Tab(text: 'Cancelled'),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: const [
          _OrderList(filter: _FilterType.active),
          _OrderList(filter: _FilterType.completed),
          _OrderList(filter: _FilterType.cancelled),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Filter enum
// ─────────────────────────────────────────────────────────────────────────────

enum _FilterType { active, completed, cancelled }

extension on _FilterType {
  bool matches(OrderStatus status) {
    switch (this) {
      case _FilterType.active:
        return status == OrderStatus.pending ||
            status == OrderStatus.processing ||
            status == OrderStatus.shipped;
      case _FilterType.completed:
        return status == OrderStatus.delivered;
      case _FilterType.cancelled:
        return status == OrderStatus.cancelled ||
            status == OrderStatus.returned;
    }
  }

  String get emptyMessage {
    switch (this) {
      case _FilterType.active:
        return 'No active orders';
      case _FilterType.completed:
        return 'No completed orders';
      case _FilterType.cancelled:
        return 'No cancelled orders';
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Order list tab
// ─────────────────────────────────────────────────────────────────────────────

class _OrderList extends ConsumerWidget {
  const _OrderList({required this.filter});
  final _FilterType filter;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncOrders = ref.watch(ordersProvider);

    return asyncOrders.when(
      loading: () => const Center(
        child: CircularProgressIndicator(
            color: Color(0xFFFF2D55), strokeWidth: 2),
      ),
      error: (err, _) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.error_outline,
                color: Color(0xFFFF2D55), size: 48),
            const SizedBox(height: 12),
            Text(err.toString(),
                style: const TextStyle(color: Colors.white70)),
            TextButton(
              onPressed: () => ref.invalidate(ordersProvider),
              child: const Text('Retry',
                  style: TextStyle(color: Color(0xFFFF2D55))),
            ),
          ],
        ),
      ),
      data: (state) {
        final filtered = state.items
            .where((o) => filter.matches(o.status))
            .toList();

        if (filtered.isEmpty) {
          return Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(Icons.receipt_long_outlined,
                    color: Color(0xFF333333), size: 64),
                const SizedBox(height: 16),
                Text(
                  filter.emptyMessage,
                  style: const TextStyle(
                      color: Color(0xFF888888), fontSize: 15),
                ),
              ],
            ),
          );
        }

        return RefreshIndicator(
          color: const Color(0xFFFF2D55),
          backgroundColor: const Color(0xFF111111),
          onRefresh: () => ref.read(ordersProvider.notifier).refresh(),
          child: ListView.separated(
            padding: const EdgeInsets.all(16),
            itemCount: filtered.length,
            separatorBuilder: (_, __) => const SizedBox(height: 12),
            itemBuilder: (context, index) {
              return _OrderCard(order: filtered[index]);
            },
          ),
        );
      },
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Order card
// ─────────────────────────────────────────────────────────────────────────────

class _OrderCard extends StatelessWidget {
  const _OrderCard({required this.order});
  final OrderEntity order;

  @override
  Widget build(BuildContext context) {
    final thumbnails = order.thumbnailUrls.take(3).toList();

    return GestureDetector(
      onTap: () => Navigator.of(context).push(
        MaterialPageRoute(
          builder: (_) => OrderDetailScreen(orderId: order.id),
        ),
      ),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: const Color(0xFF111111),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Header row ──────────────────────────────────────────────
            Row(
              children: [
                // Thumbnail grid (up to 3)
                SizedBox(
                  width: thumbnails.length * 36.0 +
                      (thumbnails.length > 1
                          ? (thumbnails.length - 1) * 4.0
                          : 0),
                  height: 48,
                  child: Stack(
                    children: List.generate(thumbnails.length, (i) {
                      return Positioned(
                        left: i * 36.0,
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(6),
                          child: Container(
                            width: 48,
                            height: 48,
                            decoration: BoxDecoration(
                              border: Border.all(
                                  color: const Color(0xFF2A2A2A)),
                              borderRadius: BorderRadius.circular(6),
                            ),
                            child: thumbnails[i].isNotEmpty
                                ? Image.network(thumbnails[i],
                                    fit: BoxFit.cover,
                                    errorBuilder: (_, __, ___) =>
                                        Container(
                                            color:
                                                const Color(0xFF2A2A2A)))
                                : Container(
                                    color: const Color(0xFF2A2A2A)),
                          ),
                        ),
                      );
                    }),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Order #${order.id.substring(0, 8).toUpperCase()}',
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        '${order.itemCount} item${order.itemCount == 1 ? '' : 's'}  •  '
                        '\$${order.total.toStringAsFixed(2)}',
                        style: const TextStyle(
                            color: Color(0xFF888888), fontSize: 12),
                      ),
                    ],
                  ),
                ),
                _StatusChip(status: order.status),
              ],
            ),
            const SizedBox(height: 12),
            const Divider(
                color: Color(0xFF1A1A1A), thickness: 1, height: 1),
            const SizedBox(height: 12),
            // ── Action button ────────────────────────────────────────────
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  _formatDate(order.createdAt),
                  style: const TextStyle(
                      color: Color(0xFF555555), fontSize: 12),
                ),
                _ActionButton(order: order),
              ],
            ),
          ],
        ),
      ),
    );
  }

  String _formatDate(DateTime dt) {
    const months = [
      'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
      'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
    ];
    return '${dt.day} ${months[dt.month - 1]} ${dt.year}';
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Status chip
// ─────────────────────────────────────────────────────────────────────────────

class _StatusChip extends StatelessWidget {
  const _StatusChip({required this.status});
  final OrderStatus status;

  Color get _color {
    switch (status) {
      case OrderStatus.pending:
        return const Color(0xFFFFC107);
      case OrderStatus.processing:
        return const Color(0xFF2196F3);
      case OrderStatus.shipped:
        return const Color(0xFF9C27B0);
      case OrderStatus.delivered:
        return const Color(0xFF4CAF50);
      case OrderStatus.cancelled:
        return const Color(0xFFFF5252);
      case OrderStatus.returned:
        return const Color(0xFFFF7043);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: _color.withAlpha(30),
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: _color.withAlpha(80), width: 0.5),
      ),
      child: Text(
        status.label,
        style: TextStyle(
            color: _color,
            fontSize: 11,
            fontWeight: FontWeight.w600),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Action button per status
// ─────────────────────────────────────────────────────────────────────────────

class _ActionButton extends StatelessWidget {
  const _ActionButton({required this.order});
  final OrderEntity order;

  String get _label {
    switch (order.status) {
      case OrderStatus.shipped:
        return 'Track';
      case OrderStatus.delivered:
        return 'Review';
      case OrderStatus.cancelled:
      case OrderStatus.returned:
        return 'Buy Again';
      default:
        return 'Details';
    }
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => Navigator.of(context).push(
        MaterialPageRoute(
          builder: (_) => OrderDetailScreen(orderId: order.id),
        ),
      ),
      child: Container(
        padding:
            const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
        decoration: BoxDecoration(
          color: const Color(0xFF1A1A1A),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: const Color(0xFF333333)),
        ),
        child: Text(
          _label,
          style: const TextStyle(
              color: Colors.white,
              fontSize: 12,
              fontWeight: FontWeight.w500),
        ),
      ),
    );
  }
}
